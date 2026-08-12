package cmd

import (
	"strings"
	"testing"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/spf13/cobra"
)

func TestSelectBackupTypeMapsEachSelector(t *testing.T) {
	cases := []struct {
		name     string
		selected map[string]bool
		want     models.BackupTypes
	}{
		{name: "none", selected: nil, want: models.NoBackupType{}},
		{name: "daily", selected: map[string]bool{"daily": true}, want: models.Daily{}},
		{name: "weekly", selected: map[string]bool{"weekly": true}, want: models.Weekly{}},
		{name: "monthly", selected: map[string]bool{"monthly": true}, want: models.Monthly{}},
		{name: "mirror", selected: map[string]bool{"mirror": true}, want: models.Mirror{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectBackupType(tc.selected)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("selectBackupType = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestSelectBackupTypeRejectsConflictingSelectors(t *testing.T) {
	cases := []struct {
		name     string
		selected map[string]bool
		want     []string
	}{
		{name: "mirror and daily", selected: map[string]bool{"mirror": true, "daily": true}, want: []string{"daily", "mirror"}},
		{name: "mirror and weekly", selected: map[string]bool{"mirror": true, "weekly": true}, want: []string{"weekly", "mirror"}},
		{name: "mirror and monthly", selected: map[string]bool{"mirror": true, "monthly": true}, want: []string{"monthly", "mirror"}},
		{name: "daily and weekly", selected: map[string]bool{"daily": true, "weekly": true}, want: []string{"daily", "weekly"}},
		{name: "all four", selected: map[string]bool{"daily": true, "weekly": true, "monthly": true, "mirror": true}, want: []string{"daily", "weekly", "monthly", "mirror"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectBackupType(tc.selected)
			if err == nil {
				t.Fatalf("selectBackupType = %#v, want the conflict rejected", got)
			}
			if _, ok := got.(models.NoBackupType); !ok {
				t.Fatalf("selectBackupType = %#v on a conflict, want no backup type", got)
			}
			for _, flag := range tc.want {
				if !strings.Contains(err.Error(), flag) {
					t.Fatalf("error = %q, want it to name %s", err, flag)
				}
			}
		})
	}
}

// The selector has to exist on both commands that narrow history by type,
// otherwise mirror rows can be viewed but never reset on their own.
func TestUsageCommandsDeclareEverySelector(t *testing.T) {
	for _, cmd := range []*cobra.Command{viewUsageCmd, resetUsageCmd} {
		for _, selector := range backupTypeSelectors {
			if cmd.Flags().Lookup(selector.flag) == nil {
				t.Fatalf("%s does not declare --%s", cmd.CommandPath(), selector.flag)
			}
		}
	}
}

// parseBuilderTypeFlags reads the flags of the command it is given, so a real
// invocation of either command has to reach models.Mirror{}.
func TestParseBuilderTypeFlagsReadsMirror(t *testing.T) {
	for _, cmd := range []*cobra.Command{viewUsageCmd, resetUsageCmd} {
		parseFlags(t, cmd, "--mirror")

		if got := parseBuilderTypeFlags(cmd); got != models.BackupTypes(models.Mirror{}) {
			t.Fatalf("%s --mirror selected %#v, want models.Mirror{}", cmd.CommandPath(), got)
		}
	}
}
