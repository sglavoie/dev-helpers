package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
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
