package mirror

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A signal cancels the context the mirror runs under, and rsync has to go with
// it instead of transferring on in the background.
func TestOSStreamerTerminatesACancelledCommand(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var out bytes.Buffer
	streamer := osStreamer{stdout: &out, stderr: &out}

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	code, err := streamer.Stream(ctx, Command{Argv: []string{"sleep", "30"}})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Stream() = %v, want the termination reported through the exit code", err)
	}
	if code == 0 {
		t.Fatal("exit code = 0, want a terminated command to be reported as such")
	}
	if elapsed >= killGrace {
		t.Fatalf("the command took %s to die, want it terminated as soon as the context was cancelled", elapsed)
	}
}

// A real transfer is watched rather than parsed, so its two streams have to
// reach the writers the command layer handed over and stay separate.
func TestOSStreamerWritesToTheGivenWriters(t *testing.T) {
	var stdout, stderr bytes.Buffer
	streamer := osStreamer{stdout: &stdout, stderr: &stderr}

	if _, err := streamer.Stream(context.Background(), Command{Argv: []string{"sh", "-c", "echo progress; echo trouble >&2"}}); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "progress\n" {
		t.Fatalf("stdout = %q, want the command's own output", got)
	}
	if got := stderr.String(); got != "trouble\n" {
		t.Fatalf("stderr = %q, want the command's own errors", got)
	}
}

func TestOSStreamerReportsTheExitCode(t *testing.T) {
	var out bytes.Buffer
	streamer := osStreamer{stdout: &out, stderr: &out}

	code, err := streamer.Stream(context.Background(), Command{Argv: []string{"sh", "-c", "exit 23"}})
	if err != nil {
		t.Fatalf("Stream() = %v, want an exit code rather than an error", err)
	}
	if code != 23 {
		t.Fatalf("exit code = %d, want 23", code)
	}
}

// The whole symlink-escape refusal rests on Resolve reporting where a path
// really is, including when the symlink is a parent rather than the leaf.
func TestOSFileSystemResolveFollowsSymlinks(t *testing.T) {
	root := t.TempDir()
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(real, "outside")
	link := filepath.Join(real, "link")
	if err := os.MkdirAll(filepath.Join(target, "Media"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	resolved, err := osFileSystem{}.Resolve(filepath.Join(link, "Media"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(target, "Media"); resolved != want {
		t.Fatalf("Resolve() = %q, want %q", resolved, want)
	}
}

// A binding is what the transfer is given instead of a path, so the name it
// returns has to keep meaning the directory that was opened after that
// directory has been renamed away and something else put in its place.
func TestOSBinderNamesTheDirectoryItOpened(t *testing.T) {
	root := t.TempDir()
	endpoint := filepath.Join(root, "endpoint")
	escape := filepath.Join(root, "escape")

	writeFile(t, filepath.Join(endpoint, "inside.txt"), "bound")
	writeFile(t, filepath.Join(escape, "outside.txt"), "elsewhere")

	bound, err := osBinder{}.Bind(endpoint, identityOfPath(t, endpoint))
	if err != nil {
		t.Fatal(err)
	}
	defer bound.Release()

	if err := os.Rename(endpoint, filepath.Join(root, "moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(escape, endpoint); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(bound.Path, "inside.txt"), "bound")
	assertAbsent(t, filepath.Join(bound.Path, "outside.txt"))
}

// An endpoint that is a symlink by the time it is bound is refused rather than
// followed, since the directory it leads to is not the one that was validated.
func TestOSBinderRefusesAnEndpointReplacedByASymlink(t *testing.T) {
	root := t.TempDir()
	endpoint := filepath.Join(root, "endpoint")
	escape := filepath.Join(root, "escape")

	writeFile(t, filepath.Join(escape, "outside.txt"), "elsewhere")
	if err := os.Symlink(escape, endpoint); err != nil {
		t.Fatal(err)
	}

	if _, err := (osBinder{}).Bind(endpoint, identityOfPath(t, escape)); err == nil {
		t.Fatal("a symlinked endpoint was bound, want it refused")
	}
}

// O_NOFOLLOW refuses a symlink but has nothing to say about an ordinary
// directory moved onto the endpoint, and the path names it just as it named the
// validated one. Only the identity the preflight recorded tells them apart.
func TestOSBinderRefusesADirectoryMovedOntoTheEndpoint(t *testing.T) {
	root := t.TempDir()
	endpoint := filepath.Join(root, "endpoint")
	substitute := filepath.Join(root, "substitute")

	writeFile(t, filepath.Join(endpoint, "inside.txt"), "bound")
	writeFile(t, filepath.Join(substitute, "elsewhere.txt"), "not reviewed")
	validated := identityOfPath(t, endpoint)

	if err := os.Rename(endpoint, filepath.Join(root, "moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(substitute, endpoint); err != nil {
		t.Fatal(err)
	}

	if _, err := (osBinder{}).Bind(endpoint, validated); err == nil {
		t.Fatal("a substituted directory was bound, want it refused")
	}
}

func identityOfPath(t *testing.T, path string) string {
	t.Helper()

	identity, err := osDevices{}.Identity(path)
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

// The mirror creates no destination, so the interval between a creation and the
// first observation of what it made does not exist: there is nothing to
// substitute a directory for. What replaces the creation is a refusal, and this
// is the production-filesystem proof of it — an absent destination stops the
// mirror before anything is read, and the leaf is still absent afterwards.
func TestOSDirectoryMakerRefusesReplacementBeforeInitialIdentityCapture(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "Source")
	writeFile(t, filepath.Join(source, "a file.txt"), "content")
	leaf := filepath.Join(root, "Destination")

	cfg := Config{Source: source, Destination: leaf, RsyncBinary: "rsync"}
	if _, err := Validate(cfg, OSDeps()); err == nil {
		t.Fatal("an absent mirror destination was accepted, want it refused")
	} else {
		requireErrorContains(t, err, "create it yourself")
	}

	assertAbsent(t, leaf)
}

// Generation counts are not read at all: their documented contract supports
// equality with an earlier value and nothing more, and the retained identity of
// a created directory needed more than that. Nothing in the package creates a
// directory either, which is what makes the guarantee hold rather than the
// arithmetic that used to.
func TestOSDirectoryMakerDoesNotInferModificationCountFromOpaqueGenerationValues(t *testing.T) {
	forbidden := []string{"GEN_COUNT", "GenCount", "getattrlist", "Mkdir", "Mkdirat"}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range forbidden {
			if strings.Contains(string(source), token) {
				t.Fatalf("%s mentions %s, want a package that neither creates a destination nor reads a generation count", name, token)
			}
		}
	}
}
