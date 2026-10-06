package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var completionCmd = &cobra.Command{
	Annotations: withProfileResolution(profileNotRequired),
	Use:         "completion [bash|zsh|fish|powershell]",
	Short:       "Generate shell completion script",
	Long: `Generate a shell completion script for goback.

To load completions:

  Bash:
    source <(goback completion bash)

  Zsh:
    goback completion zsh > "${fpath[1]}/_goback"

  Fish:
    goback completion fish | source`,
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		switch args[0] {
		case "bash":
			err = RootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			err = RootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			err = RootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			err = RootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		cobra.CheckErr(err)
	},
}

func init() {
	RootCmd.AddCommand(completionCmd)
}

// Completion reads its own configuration without prompting, creating files,
// resolving a hostname, or changing the application's Viper state.
func completeProfiles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	path, _ := cmd.Flags().GetString("config")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		path = filepath.Join(home, ".goback.json")
	}
	settings := viper.New()
	settings.SetConfigFile(path)
	if err := settings.ReadInConfig(); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for name := range settings.GetStringMap("profiles") {
		if strings.HasPrefix(name, toComplete) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, cobra.ShellCompDirectiveNoFileComp
}
