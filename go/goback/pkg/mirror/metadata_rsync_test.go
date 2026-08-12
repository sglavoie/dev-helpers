package mirror

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The mirror promises full supported APFS metadata and refuses to run on an
// rsync that cannot carry it. These tests check the promise end to end on a
// disposable fixture, and skip only when the filesystem under the fixture
// cannot hold the metadata in the first place.

// runTool runs a macOS metadata tool and returns its combined output.
func runTool(t *testing.T, argv ...string) (string, error) {
	t.Helper()

	output, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
	return string(output), err
}

// requireMetadataSupport runs the setup command that puts the metadata on the
// fixture and skips the test, naming the filesystem, when it cannot hold it.
func requireMetadataSupport(t *testing.T, what, dir string, argv ...string) {
	t.Helper()

	if output, err := runTool(t, argv...); err != nil {
		t.Skipf("unsupported filesystem: %s cannot hold %s (%v: %s)", dir, what, err, strings.TrimSpace(output))
	}
}

const testXattrName = "com.sglavoie.goback.test"

func TestMirrorPreservesExtendedAttributes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	file := filepath.Join(source, "tagged.mov")
	writeFile(t, file, "content")
	requireMetadataSupport(t, "extended attributes", root, "xattr", "-w", testXattrName, "kept", file)

	makeDir(t, destination)

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, _, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded: %v", result.Status, result.Err)
	}

	value, err := runTool(t, "xattr", "-p", testXattrName, filepath.Join(destination, "tagged.mov"))
	if err != nil {
		t.Fatalf("the mirrored file carries no %s: %v (%s)", testXattrName, err, value)
	}
	if strings.TrimSpace(value) != "kept" {
		t.Fatalf("%s = %q on the mirrored file, want %q", testXattrName, strings.TrimSpace(value), "kept")
	}
}

func TestMirrorPreservesAccessControlLists(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	file := filepath.Join(source, "restricted.mov")
	writeFile(t, file, "content")
	requireMetadataSupport(t, "access control lists", root, "chmod", "+a", "everyone allow read", file)

	makeDir(t, destination)

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, _, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded: %v", result.Status, result.Err)
	}

	if got, want := aclEntries(t, filepath.Join(destination, "restricted.mov")), aclEntries(t, file); got != want {
		t.Fatalf("the mirrored file lists the ACL %q, want the source's %q", got, want)
	}
}

// aclEntries returns the numbered ACL entries ls prints under a path, which is
// everything the mirror has to reproduce.
func aclEntries(t *testing.T, path string) string {
	t.Helper()

	listing, err := runTool(t, "ls", "-le", path)
	if err != nil {
		t.Fatalf("ls -le failed on %s: %v (%s)", path, err, listing)
	}

	var entries []string
	for _, line := range strings.Split(listing, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "0:") || strings.HasPrefix(trimmed, "1:") {
			entries = append(entries, trimmed)
		}
	}
	if len(entries) == 0 {
		t.Fatalf("%s lists no ACL entry at all:\n%s", path, listing)
	}
	return strings.Join(entries, "\n")
}

// creationTime reads the birth time macOS records for a path.
func creationTime(t *testing.T, path string) string {
	t.Helper()

	output, err := runTool(t, "stat", "-f", "%B", path)
	if err != nil {
		t.Fatalf("stat -f %%B failed on %s: %v (%s)", path, err, output)
	}
	return strings.TrimSpace(output)
}

// A creation time is only preserved if it survives being copied later than it
// was created, so the fixture is deliberately older than its transfer.
func TestMirrorPreservesCreationTimes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	file := filepath.Join(source, "old.mov")
	writeFile(t, file, "content")

	born := creationTime(t, file)
	if born == "0" {
		t.Skipf("unsupported filesystem: %s records no creation time", root)
	}
	time.Sleep(1100 * time.Millisecond)

	makeDir(t, destination)

	cfg, deps := realRsyncDeps(t, source, destination)
	exec, _, out := realExecDeps(true)

	result, err := Mirror(context.Background(), cfg, deps, exec, out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if result.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded: %v", result.Status, result.Err)
	}

	if copied := creationTime(t, filepath.Join(destination, "old.mov")); copied != born {
		t.Fatalf("creation time = %s on the mirrored file, want the source's %s", copied, born)
	}
}
