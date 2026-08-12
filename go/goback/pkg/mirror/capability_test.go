package mirror

import (
	"context"
	"fmt"
	"testing"
)

func TestProbeRsyncAcceptsACapableBuild(t *testing.T) {
	cfg, deps, runner := mountedSetup()

	if err := ProbeRsync(context.Background(), cfg, deps); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %v, want a single --version probe", runner.calls)
	}
	want := []string{"rsync", "--version"}
	if runner.calls[0][0] != want[0] || runner.calls[0][1] != want[1] {
		t.Fatalf("probe = %v, want %v", runner.calls[0], want)
	}
}

// The rsync shipped with macOS silently drops metadata the mirror promises to
// copy, so a build without those capabilities is rejected by name.
func TestProbeRsyncRejectsAnIncapableBuild(t *testing.T) {
	cases := []struct {
		name     string
		version  string
		wantMiss string
	}{
		{
			name: "no ACLs or extended attributes",
			version: "rsync  version 2.6.9  protocol version 29\nCapabilities:\n" +
				"    64-bit files, socketpairs, hard links, symlinks, batchfiles,\n" +
				"    inplace, IPv6, 64-bit system inums, 64-bit internal inums\n",
			wantMiss: "acls",
		},
		{
			name: "no creation times",
			version: "rsync  version 3.2.7  protocol version 31\nCapabilities:\n" +
				"    hardlinks, ACLs, xattrs, no crtimes\n",
			wantMiss: "crtimes",
		},
		{
			name: "no hard links",
			version: "rsync  version 3.4.4  protocol version 32\nCapabilities:\n" +
				"    no hardlinks, ACLs, xattrs, crtimes\n",
			wantMiss: "hardlinks",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, deps, runner := mountedSetup()
			runner.version = CommandResult{Stdout: tc.version}

			err := ProbeRsync(context.Background(), cfg, deps)
			requireErrorContains(t, err, tc.wantMiss)
			requireErrorContains(t, err, "install an rsync 3")
		})
	}
}

func TestProbeRsyncRejectsAnUnreadableVersion(t *testing.T) {
	cfg, deps, runner := mountedSetup()
	runner.version = CommandResult{Stdout: "openrsync: protocol version 29\n"}

	err := ProbeRsync(context.Background(), cfg, deps)
	requireErrorContains(t, err, "does not report its capabilities")
}

func TestProbeRsyncReportsAMissingBinary(t *testing.T) {
	cfg, deps, runner := mountedSetup()
	cfg.RsyncBinary = "rsync3"
	runner.versionErr = fmt.Errorf("rsync3 not found in PATH: %w", errFake)

	err := ProbeRsync(context.Background(), cfg, deps)
	requireErrorContains(t, err, "rsync3 not found in PATH")
}

func TestProbeRsyncReportsAFailingBinary(t *testing.T) {
	cfg, deps, runner := mountedSetup()
	runner.version = CommandResult{ExitCode: 2, Stderr: "rsync: illegal option"}

	err := ProbeRsync(context.Background(), cfg, deps)
	requireErrorContains(t, err, "exit code 2")
	requireErrorContains(t, err, "rsync: illegal option")
}
