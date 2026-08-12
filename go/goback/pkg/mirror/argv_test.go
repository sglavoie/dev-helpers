package mirror

import (
	"slices"
	"strings"
	"testing"
)

// testRulesPath stands for the throwaway file a real transfer reads its
// deletion boundary from.
const testRulesPath = "/tmp/goback-mirror-rules-test"

func TestArgvUsesContentSemantics(t *testing.T) {
	cases := []struct {
		name        string
		source      string
		destination string
		wantSource  string
		wantDest    string
	}{
		{
			name:        "plain paths",
			source:      "/Volumes/SanDisk/Media",
			destination: "/Volumes/Elements/Media",
			wantSource:  "/Volumes/SanDisk/Media/",
			wantDest:    "/Volumes/Elements/Media",
		},
		{
			name:        "trailing slashes are normalized",
			source:      "/Volumes/SanDisk/Media/",
			destination: "/Volumes/Elements/Media/",
			wantSource:  "/Volumes/SanDisk/Media/",
			wantDest:    "/Volumes/Elements/Media",
		},
		{
			name:        "spaces and quotes are passed through unquoted",
			source:      "/Volumes/My Disk/A \"Media\" Folder",
			destination: "/Volumes/Other/It's Media",
			wantSource:  "/Volumes/My Disk/A \"Media\" Folder/",
			wantDest:    "/Volumes/Other/It's Media",
		},
		{
			name:        "glob characters are passed through unexpanded",
			source:      "/Volumes/SanDisk/Media*",
			destination: "/Volumes/Elements/$HOME;rm -rf",
			wantSource:  "/Volumes/SanDisk/Media*/",
			wantDest:    "/Volumes/Elements/$HOME;rm -rf",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			argv := DryRunArgv(Config{Source: tc.source, Destination: tc.destination, RsyncBinary: "rsync"})

			if argv[0] != "rsync" {
				t.Fatalf("argv[0] = %q, want the configured binary", argv[0])
			}
			if got := argv[len(argv)-2]; got != tc.wantSource {
				t.Fatalf("source argument = %q, want %q", got, tc.wantSource)
			}
			if got := argv[len(argv)-1]; got != tc.wantDest {
				t.Fatalf("destination argument = %q, want %q", got, tc.wantDest)
			}
		})
	}
}

func TestArgvCarriesTheFixedMirrorPolicy(t *testing.T) {
	argv := DryRunArgv(Config{Source: testSource, Destination: testDestination, RsyncBinary: "rsync"})

	required := []string{
		"--archive",
		"--hard-links",
		"--acls",
		"--xattrs",
		"--crtimes",
		"--delete",
		"--delete-delay",
		"--partial-dir=" + PartialDir,
		"--out-format=%i %n",
		"--stats",
		"--dry-run",
	}
	for _, flag := range required {
		if !slices.Contains(argv, flag) {
			t.Fatalf("argv = %v, want it to contain %q", argv, flag)
		}
	}

	// Each of these would either slow the mirror down enormously, defeat
	// rsync's protection against deleting after a read error, or leave a
	// destination file half-written.
	for _, flag := range []string{"--checksum", "--ignore-errors", "--inplace", "--delete-during", "--delete-before"} {
		if slices.Contains(argv, flag) {
			t.Fatalf("argv = %v, want it to omit %q", argv, flag)
		}
	}
}

func TestArgvOmitsDryRunForARealMirror(t *testing.T) {
	argv := TransferArgv(Config{Source: testSource, Destination: testDestination, RsyncBinary: "rsync"}, testRulesPath)

	if slices.Contains(argv, "--dry-run") {
		t.Fatalf("argv = %v, want no --dry-run when the mirror is real", argv)
	}
	for _, flag := range []string{"--archive", "--hard-links", "--acls", "--xattrs", "--crtimes", "--delete", "--delete-delay", "--partial-dir=" + PartialDir} {
		if !slices.Contains(argv, flag) {
			t.Fatalf("argv = %v, want the same fixed policy as a dry run, including %q", argv, flag)
		}
	}
}

// Progress belongs to a transfer somebody is watching. The statistics of a dry
// run are parsed instead, and a human-readable byte count would not parse.
func TestArgvShowsProgressOnlyForARealMirror(t *testing.T) {
	cfg := Config{Source: testSource, Destination: testDestination, RsyncBinary: "rsync"}

	real := TransferArgv(cfg, testRulesPath)
	dry := DryRunArgv(cfg)

	for _, flag := range []string{"--info=progress2", "--human-readable"} {
		if !slices.Contains(real, flag) {
			t.Fatalf("real argv = %v, want it to contain %q", real, flag)
		}
		if slices.Contains(dry, flag) {
			t.Fatalf("dry run argv = %v, want it to omit %q", dry, flag)
		}
	}
}

// A real transfer scans the destination itself, after every look the mirror
// took, so it never runs without the boundary that tells it what it may delete.
func TestTransferArgvAlwaysCarriesTheDeletionBoundary(t *testing.T) {
	cfg := Config{Source: testSource, Destination: testDestination, RsyncBinary: "rsync"}

	real := TransferArgv(cfg, testRulesPath)
	dry := DryRunArgv(cfg)

	for _, flag := range []string{"--from0", "--filter=merge " + testRulesPath} {
		if !slices.Contains(real, flag) {
			t.Fatalf("real argv = %v, want it to contain %q", real, flag)
		}
		if slices.Contains(dry, flag) {
			t.Fatalf("dry run argv = %v, want it to omit %q", dry, flag)
		}
	}
	if index := slices.Index(real, "--filter=merge "+testRulesPath); index > len(real)-3 {
		t.Fatalf("argv = %v, want the boundary before the two endpoints", real)
	}
}

// rsync reads a merged rule file as null-delimited under --from0, and a rule
// names one path rather than a pattern that looks like it.
func TestDeletionRulesRiskTheReviewedPathsAndProtectEverythingElse(t *testing.T) {
	rules := string(DeletionRules([]string{"gone.txt", "sub/star*and?.txt", "newline\nhere.txt", `back\slash.txt`}))

	want := "R /gone.txt\x00" +
		`R /sub/star\*and\?.txt` + "\x00" +
		"R /newline\nhere.txt\x00" +
		`R /back\slash.txt` + "\x00" +
		"P /**\x00"
	if rules != want {
		t.Fatalf("DeletionRules() = %q, want %q", rules, want)
	}
}

// An empty destination has nothing anybody reviewed, so the transfer may delete
// nothing at all.
func TestDeletionRulesProtectEverythingWhenNothingWasReviewed(t *testing.T) {
	if got, want := string(DeletionRules(nil)), "P /**\x00"; got != want {
		t.Fatalf("DeletionRules(nil) = %q, want %q", got, want)
	}
}

func TestFormatArgvQuotesOnlyAmbiguousArguments(t *testing.T) {
	got := FormatArgv([]string{"rsync", "--archive", "/Volumes/My Disk/Media/", "/Volumes/Other/Media"})
	want := `rsync --archive "/Volumes/My Disk/Media/" /Volumes/Other/Media`
	if got != want {
		t.Fatalf("FormatArgv() = %q, want %q", got, want)
	}
	if strings.Contains(got, "\n") {
		t.Fatal("FormatArgv() produced a multi-line command, want a single line")
	}
}
