package buildcmd

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/destinationlock"
	"github.com/spf13/viper"
)

func verificationFixture(t *testing.T) (string, string, string) {
	t.Helper()
	src, dest, log := snapshotFixture(t)
	if err := os.Mkdir(filepath.Join(dest, "daily"), 0700); err != nil {
		t.Fatal(err)
	}
	return src, dest, log
}

func TestVerificationAlwaysChecksumsWithoutTransferOrDeletionFlags(t *testing.T) {
	src, dest, log := verificationFixture(t)
	for _, key := range []string{"delete", "deleteExcluded", "force", "ignoreErrors", "hardLinks", "dryRun", "verbose", "pruneEmptyDirs"} {
		viper.Set("profiles.test.rsync.daily."+key, true)
	}
	viper.Set("showProgress", true)
	viper.Set("cliVerbose", true)
	pattern := "a 'quoted' $HOME `command` $(command) *\nnext"
	viper.Set("profiles.test.rsync.daily.excludedPatterns", []string{pattern})
	report, err := VerifyDaily(context.Background())
	if err != nil || !report.Completed || len(report.Differences) != 0 || len(report.Errors) != 0 {
		t.Fatalf("report=%+v, err=%v", report, err)
	}
	args := readArgs(t, log)
	for _, required := range []string{"--dry-run", "--ignore-times", "--prune-empty-dirs", "--exclude=" + pattern} {
		if !slices.Contains(args, required) {
			t.Fatalf("verification omitted required argument %q: %q", required, args)
		}
	}
	for _, forbidden := range []string{"--delete", "--delete-excluded", "--force", "--ignore-errors", "--progress", "--verbose", "--stats"} {
		if slices.Contains(args, forbidden) {
			t.Fatalf("verification inherited backup argument %q: %q", forbidden, args)
		}
	}
	if !slices.Equal(args[len(args)-3:], []string{"--", src, filepath.Join(dest, "daily")}) {
		t.Fatalf("verification changed local endpoints: %q", args)
	}
	requireAbsent(t, filepath.Join(os.Getenv("HOME"), ".goback.db"))
	requireAbsent(t, filepath.Join(os.Getenv("HOME"), ".goback"))
}

func TestVerificationLocksEvenWhenConfigurationSetsDryRun(t *testing.T) {
	_, dest, log := verificationFixture(t)
	viper.Set("profiles.test.rsync.daily.dryRun", true)
	_, release, err := destinationlock.Acquire(context.Background(), dest)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	report, err := VerifyDaily(context.Background())
	if err == nil || report.Completed || !strings.Contains(err.Error(), "another goback operation") || len(report.Errors) != 1 {
		t.Fatalf("report=%+v, err=%v", report, err)
	}
	requireAbsent(t, log)
}

func TestVerificationRejectsAbsentAndEscapingBackupWithoutStartingRsync(t *testing.T) {
	for _, name := range []string{"absent", "escape", "file", "overlap", "source absent"} {
		t.Run(name, func(t *testing.T) {
			src, dest, log := snapshotFixture(t)
			daily := filepath.Join(dest, "daily")
			var err error
			switch name {
			case "escape":
				err = os.Symlink(t.TempDir(), daily)
			case "file":
				err = os.WriteFile(daily, nil, 0600)
			case "overlap":
				err = os.Symlink(src, daily)
			case "source absent":
				err = os.RemoveAll(src)
			}
			if err != nil {
				t.Fatal(err)
			}
			report, err := VerifyDaily(context.Background())
			if err == nil || report.Completed || len(report.Errors) == 0 {
				t.Fatalf("report=%+v, err=%v", report, err)
			}
			requireAbsent(t, log)
			requireAbsent(t, filepath.Join(os.Getenv("HOME"), ".goback.db"))
		})
	}
}

func TestVerificationNeverAcceptsPartialScans(t *testing.T) {
	for _, tc := range []struct{ name, script, want string }{
		{"unreadable", "printf '>fc........\\tfile\\n'\necho 'file: Permission denied' >&2\nexit 23", "Permission denied"},
		{"vanished", "exit 24", "exit status 24"},
		{"unexpected output", "echo 'skipping an unreadable file'", "unrecognized"},
		{"stderr with zero exit", "echo 'file could not be read' >&2", "could not be read"},
		{"missing executable", "", "executable file not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, log := verificationFixture(t)
			program := filepath.Join(filepath.Dir(log), "rsync")
			if tc.script == "" {
				t.Setenv("PATH", t.TempDir())
			} else if err := os.WriteFile(program, []byte("#!/bin/sh\n"+tc.script+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			report, err := VerifyDaily(context.Background())
			if err == nil || report.Completed || len(report.Errors) == 0 || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("report=%+v, err=%v", report, err)
			}
			if tc.name == "unreadable" && len(report.Differences) != 1 {
				t.Fatalf("lost partial differences: %+v", report)
			}
			requireAbsent(t, filepath.Join(os.Getenv("HOME"), ".goback"))
		})
	}
}

func TestVerificationCancellationReleasesLock(t *testing.T) {
	_, dest, log := verificationFixture(t)
	program := "#!/bin/sh\nprintf started > \"$GOBACK_TEST_ARGS\"\nexec sleep 30\n"
	if err := os.WriteFile(filepath.Join(filepath.Dir(log), "rsync"), []byte(program), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, err := os.Stat(log); err == nil {
				cancel()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()
	report, err := VerifyDaily(ctx)
	<-done
	if err == nil || report.Completed || !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("report=%+v, err=%v", report, err)
	}
	_, release, err := destinationlock.Acquire(context.Background(), dest)
	if err != nil {
		t.Fatalf("lock retained after cancellation: %v", err)
	}
	release()
}

func TestVerificationRejectsEndpointReplacementDuringScan(t *testing.T) {
	for _, endpoint := range []string{"source", "daily", "root"} {
		t.Run(endpoint, func(t *testing.T) {
			src, dest, log := verificationFixture(t)
			path := map[string]string{"source": src, "daily": filepath.Join(dest, "daily"), "root": dest}[endpoint]
			t.Setenv("GOBACK_TEST_ENDPOINT", path)
			program := "#!/bin/sh\nmv \"$GOBACK_TEST_ENDPOINT\" \"$GOBACK_TEST_ENDPOINT.old\"\nmkdir \"$GOBACK_TEST_ENDPOINT\"\n"
			if err := os.WriteFile(filepath.Join(filepath.Dir(log), "rsync"), []byte(program), 0700); err != nil {
				t.Fatal(err)
			}
			report, err := VerifyDaily(context.Background())
			if err == nil || report.Completed || !strings.Contains(err.Error(), "changed") {
				t.Fatalf("report=%+v, err=%v", report, err)
			}
		})
	}
}

func TestParseVerificationPreservesNamesAndSeparatesMissingFromTypeChanges(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	for _, name := range []string{" leading space", "missing", "wrong-type", "line\nbreak", `literal\#012`, "metadata"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
		if name != "missing" && name != "wrong-type" {
			content := "modified"
			if name == "metadata" {
				content = "original"
			}
			if err := os.WriteFile(filepath.Join(dest, name), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.Mkdir(filepath.Join(dest, "wrong-type"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("absent", filepath.Join(dest, "dangling")); err != nil {
		t.Fatal(err)
	}
	output := ">fc........\t leading space\n>f+++++++++\tmissing\n>f+++++++++\twrong-type\ncd+++++++++\tempty/\ncL+++++++++\tdangling\n>fc........\tline\\#012break\n>fc........\tliteral\\#134#012\n.f..tp.....\tmetadata\n.d..t......\t./\n"
	got, err := compareVerification(context.Background(), output, source+"/", dest)
	want := []VerificationDifference{
		{" leading space", "different"}, {"dangling", "different"}, {"empty/", "missing"},
		{"line\nbreak", "different"}, {`literal\#012`, "different"}, {"missing", "missing"}, {"wrong-type", "different"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("differences=%+v, err=%v; want=%+v", got, err, want)
	}
	for _, line := range []string{"bad", ">f?????????\tfile", ">fc........\t/absolute", ">fc........\t../escape", ">fc........\tnull\\#000byte", "*deleting  \tfile"} {
		if _, err := compareVerification(context.Background(), line+"\n", source+"/", dest); err == nil {
			t.Fatalf("accepted unsafe or ambiguous output %q", line)
		}
	}
}
