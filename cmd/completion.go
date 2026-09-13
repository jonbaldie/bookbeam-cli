package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script for your terminal",
	Long: `To load completions:

Bash:
  $ source <(bookbeam completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ bookbeam completion bash > /etc/bash_completion.d/bookbeam
  # macOS:
  $ bookbeam completion bash > $(brew --prefix)/etc/bash_completion.d/bookbeam

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ bookbeam completion zsh > "${fpath[1]}/_bookbeam"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ bookbeam completion fish | source

  # To load completions for each session, execute once:
  $ bookbeam completion fish > ~/.config/fish/completions/bookbeam.fish
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell type %s", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
