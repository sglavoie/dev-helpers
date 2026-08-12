package mirror

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// realRsyncDeps runs against the machine's own rsync and filesystem, with only
// the device identities faked: two disposable directories are necessarily on
// the same filesystem, while a real mirror never is.
func realRsyncDeps(t *testing.T, source, destination string) (Config, Deps) {
	t.Helper()

	cfg := Config{Source: source, Destination: destination, RsyncBinary: "rsync"}
	deps := OSDeps()
	deps.Devices = realDevices{fakeDevices{devices: map[string]string{
		"/":         "system",
		source:      "source-device",
		destination: "destination-device",
	}}}

	if err := ProbeRsync(context.Background(), cfg, deps); err != nil {
		t.Skipf("skipping the real rsync test: %v", err)
	}
	return cfg, deps
}

// realDevices fakes which filesystem a path is on but answers for the
// directories themselves from the machine, since the binding those identities
// are compared against opens the real endpoints.
type realDevices struct {
	fakeDevices
}

func (realDevices) Identity(path string) (string, error) {
	return osDevices{}.Identity(path)
}

func makeDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDryRunAgainstRealRsync(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "a file.txt"), "hello")
	writeFile(t, filepath.Join(source, `we$ird'"na me.txt`), "x")
	writeFile(t, filepath.Join(source, "sub dir", "nested.txt"), "y")
	writeFile(t, filepath.Join(source, "changed.txt"), "new content")

	writeFile(t, filepath.Join(destination, "changed.txt"), "old")
	writeFile(t, filepath.Join(destination, "gone dir", "obsolete file.txt"), "z")

	// The destination root is written after the source root, so their
	// modification times can land in different seconds, and rsync then
	// itemizes "./" as an update of its own. What is under test is the
	// itemization of the fixtures, not where the clock happened to be.
	matchModTime(t, source, destination)

	cfg, deps := realRsyncDeps(t, source, destination)

	plan, err := DryRun(context.Background(), cfg, deps)
	if err != nil {
		t.Fatal(err)
	}

	if plan.Changes.Created != 4 {
		t.Fatalf("Created = %d, want the three new files and their directory: %#v", plan.Changes.Created, plan.Changes)
	}
	if plan.Changes.Updated != 1 {
		t.Fatalf("Updated = %d, want only changed.txt: %#v", plan.Changes.Updated, plan.Changes)
	}
	deleted := make([]string, 0, len(plan.Changes.Deletions))
	for _, deletion := range plan.Changes.Deletions {
		deleted = append(deleted, deletion.Path)
	}
	for _, want := range []string{"gone dir/", "gone dir/obsolete file.txt"} {
		if !slices.Contains(deleted, want) {
			t.Fatalf("deletions = %v, want %q among them", deleted, want)
		}
	}
	if plan.Changes.TransferBytes == 0 {
		t.Fatal("TransferBytes = 0, want the size of the files that would be sent")
	}

	// The dry run must have left both endpoints exactly as they were.
	assertFileContent(t, filepath.Join(destination, "changed.txt"), "old")
	assertExists(t, filepath.Join(destination, "gone dir", "obsolete file.txt"))
	assertAbsent(t, filepath.Join(destination, "a file.txt"))
	assertAbsent(t, filepath.Join(destination, "sub dir"))
	assertAbsent(t, filepath.Join(destination, PartialDir))
}

// A destination that does not exist is refused by the preflight, so rsync is
// never given one to create.
func TestDryRunLeavesAMissingDestinationLeafAbsent(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	writeFile(t, filepath.Join(source, "a file.txt"), "hello")

	cfg, deps := realRsyncDeps(t, source, destination)

	_, err := DryRun(context.Background(), cfg, deps)
	requireErrorContains(t, err, "create it yourself")
	assertAbsent(t, destination)
}

// An empty source would plan the deletion of the whole destination, so it is
// rejected before rsync is ever executed.
func TestDryRunRefusesAnEmptySourceAgainstRealRsync(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(destination, "precious.mov"), "keep me")

	cfg, deps := realRsyncDeps(t, source, destination)

	_, err := DryRun(context.Background(), cfg, deps)
	requireErrorContains(t, err, "is empty")
	assertFileContent(t, filepath.Join(destination, "precious.mov"), "keep me")
}

// matchModTime gives the destination the modification time of the source, so
// the two directories compare equal to rsync.
func matchModTime(t *testing.T, source, destination string) {
	t.Helper()
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(destination, time.Now(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}

func assertAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists after a dry run, want it untouched", path)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("%s is gone after a dry run: %v", path, err)
	}
}

// assertOnlyEntries reports what a directory holds, which is what proves that
// nothing was written into it as much as that nothing was removed from it.
func assertOnlyEntries(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	slices.Sort(want)
	if !slices.Equal(names, want) {
		t.Fatalf("%s holds %q, want exactly %q", dir, names, want)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s = %q after a dry run, want the untouched %q", path, content, want)
	}
}
