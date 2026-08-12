package mirror

import (
	"reflect"
	"testing"
)

func TestParseChangesReadsItemizationAndStatistics(t *testing.T) {
	changes, err := ParseChanges(mountedRsyncOutput)
	if err != nil {
		t.Fatal(err)
	}

	if changes.Created != 2 {
		t.Fatalf("Created = %d, want 2", changes.Created)
	}
	if changes.Updated != 1 {
		t.Fatalf("Updated = %d, want 1", changes.Updated)
	}
	if changes.Deleted != 3 {
		t.Fatalf("Deleted = %d, want 3", changes.Deleted)
	}
	if changes.TransferBytes != 2048 {
		t.Fatalf("TransferBytes = %d, want 2048 with the thousands separator removed", changes.TransferBytes)
	}
	want := []Deletion{
		{Path: "dir gone/c.txt"},
		{Path: "dir gone/", IsDir: true},
		{Path: "gone.txt"},
	}
	if !reflect.DeepEqual(changes.Deletions, want) {
		t.Fatalf("Deletions = %#v, want %#v", changes.Deletions, want)
	}
	if changes.Empty() {
		t.Fatal("Empty() = true, want false when there is work to do")
	}
}

// Every character after the eleven-character change summary and its separator
// belongs to the path, so a name containing spaces, quotes, or a dollar sign
// survives parsing intact.
func TestParseChangesKeepsDifficultFilenames(t *testing.T) {
	output := "" +
		">f+++++++++ a file with spaces.txt\n" +
		">f+++++++++ we$ird'\"na me.txt\n" +
		">f+++++++++ trailing space .txt\n" +
		">f+++++++++ sub dir/nested name.txt\n" +
		"*deleting   old dir/old \"file\".txt\n" +
		"*deleting   spaced name.mov\n" +
		"Total transferred file size: 10 bytes\n"

	changes, err := ParseChanges(output)
	if err != nil {
		t.Fatal(err)
	}
	if changes.Created != 4 {
		t.Fatalf("Created = %d, want 4", changes.Created)
	}
	want := []Deletion{
		{Path: `old dir/old "file".txt`},
		{Path: "spaced name.mov"},
	}
	if !reflect.DeepEqual(changes.Deletions, want) {
		t.Fatalf("Deletions = %#v, want %#v", changes.Deletions, want)
	}
}

// rsync escapes characters it cannot print. The escaped form is what the
// parser reports, so what is shown to the user is always printable.
func TestParseChangesKeepsEscapedPaths(t *testing.T) {
	output := "*deleting   caf\\#303\\#251 latte.mov\nTotal transferred file size: 0 bytes\n"

	changes, err := ParseChanges(output)
	if err != nil {
		t.Fatal(err)
	}
	want := []Deletion{{Path: `caf\#303\#251 latte.mov`}}
	if !reflect.DeepEqual(changes.Deletions, want) {
		t.Fatalf("Deletions = %#v, want %#v", changes.Deletions, want)
	}
}

func TestParseChangesIgnoresNoiseLines(t *testing.T) {
	output := "" +
		"created directory /Volumes/Elements/Media\n" +
		"cd+++++++++ ./\n" +
		"\n" +
		"sending incremental file list\n" +
		"rsync: some warning about a file\n" +
		"Number of deleted files: 0\n" +
		"Total file size: 999 bytes\n" +
		"Total transferred file size: 0 bytes\n" +
		"sent 208 bytes  received 80 bytes  576.00 bytes/sec\n" +
		"total size is 8  speedup is 0.03 (DRY RUN)\n"

	changes, err := ParseChanges(output)
	if err != nil {
		t.Fatal(err)
	}
	if changes.Created != 1 || changes.Updated != 0 || changes.Deleted != 0 {
		t.Fatalf("changes = %#v, want only the itemized directory counted", changes)
	}
	if changes.TransferBytes != 0 {
		t.Fatalf("TransferBytes = %d, want the transferred total rather than the file size total", changes.TransferBytes)
	}
}

func TestParseChangesReportsNoWork(t *testing.T) {
	changes, err := ParseChanges("\nTotal transferred file size: 0 bytes\n")
	if err != nil {
		t.Fatal(err)
	}
	if !changes.Empty() {
		t.Fatalf("Empty() = false for %#v, want true when rsync itemized nothing", changes)
	}
	if len(changes.Deletions) != 0 {
		t.Fatalf("Deletions = %#v, want none", changes.Deletions)
	}
}

// Without the transferred total there is no way to check the destination has
// room, so a truncated or unrecognized output must fail rather than be read as
// a zero-byte transfer.
func TestParseChangesRejectsOutputWithoutStatistics(t *testing.T) {
	_, err := ParseChanges(">f+++++++++ a.txt\n*deleting   b.txt\n")
	requireErrorContains(t, err, "Total transferred file size:")
}

func TestParseChangesClassifiesChangeSummaries(t *testing.T) {
	cases := []struct {
		name    string
		summary string
		created bool
	}{
		{name: "new file", summary: ">f+++++++++", created: true},
		{name: "new directory", summary: "cd+++++++++", created: true},
		{name: "new symlink", summary: "cL+++++++++", created: true},
		{name: "size and time change", summary: ">f.st......", created: false},
		{name: "permission change", summary: ".f...p.....", created: false},
		{name: "directory time change", summary: ".d..t......", created: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changes, err := ParseChanges(tc.summary + " item\nTotal transferred file size: 0 bytes\n")
			if err != nil {
				t.Fatal(err)
			}
			if tc.created && changes.Created != 1 {
				t.Fatalf("%q gave %#v, want one creation", tc.summary, changes)
			}
			if !tc.created && changes.Updated != 1 {
				t.Fatalf("%q gave %#v, want one update", tc.summary, changes)
			}
		})
	}
}
