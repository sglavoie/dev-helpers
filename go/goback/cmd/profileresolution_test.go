package cmd

import (
	"testing"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/spf13/cobra"
)

func withAllProfiles(t *testing.T, all bool) {
	t.Helper()

	previous := config.AllProfiles
	config.AllProfiles = all
	t.Cleanup(func() { config.AllProfiles = previous })
}

func TestExistingCommandsResolveProfiles(t *testing.T) {
	withAllProfiles(t, false)

	for _, cmd := range []*cobra.Command{dailyCmdRun, weeklyCmdRun, monthlyCmdRun, allCmdRun, usageCmd, ejectCmd} {
		if !needsProfileResolution(cmd) {
			t.Fatalf("%q does not require profile resolution, want the existing behavior preserved", cmd.CommandPath())
		}
	}
}

func TestEjectAllSkipsProfileResolution(t *testing.T) {
	withAllProfiles(t, true)

	if needsProfileResolution(ejectCmd) {
		t.Fatal("eject --all requires profile resolution, want it to load the config without selecting a profile")
	}
}

func TestRunAllStillResolvesProfiles(t *testing.T) {
	withAllProfiles(t, true)

	if !needsProfileResolution(allCmdRun) {
		t.Fatal("run all does not require profile resolution, want --all to keep iterating over profiles")
	}
}

func TestProfileNotRequiredCommandSkipsResolution(t *testing.T) {
	withAllProfiles(t, false)

	mirrorLike := &cobra.Command{Use: "mirror", Annotations: withProfileResolution(profileNotRequired)}
	RootCmd.AddCommand(mirrorLike)
	t.Cleanup(func() { RootCmd.RemoveCommand(mirrorLike) })

	if needsProfileResolution(mirrorLike) {
		t.Fatal("an unannotated-profile command requires resolution, want profileNotRequired to opt out")
	}
}

func TestSubcommandInheritsParentClassification(t *testing.T) {
	withAllProfiles(t, false)

	parent := &cobra.Command{Use: "parent", Annotations: withProfileResolution(profileNotRequired)}
	child := &cobra.Command{Use: "child"}
	parent.AddCommand(child)
	grandchild := &cobra.Command{Use: "grandchild", Annotations: withProfileResolution(profileRequired)}
	child.AddCommand(grandchild)

	if needsProfileResolution(child) {
		t.Fatal("child requires profile resolution, want it to inherit the parent's opt-out")
	}
	if !needsProfileResolution(grandchild) {
		t.Fatal("grandchild does not require profile resolution, want its own annotation to win")
	}
}
