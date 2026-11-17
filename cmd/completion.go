package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var completionShells = []string{"bash", "zsh", "fish"}

var completionCmd = &cobra.Command{
	Use:                   "completion <shell>",
	Short:                 "Generate the shell completion script",
	Args:                  cobra.ExactArgs(1),
	ValidArgsFunction:     completeShell,
	DisableFlagsInUseLine: true,
	RunE:                  runCompletion,
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

// completeShell suggests the supported shells for the first argument, and
// nothing after it.
func completeShell(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completionShells, cobra.ShellCompDirectiveNoFileComp
}

func runCompletion(cmd *cobra.Command, args []string) error {
	shell := args[0]

	w := cmd.OutOrStdout()

	switch shell {
	case "bash":
		return cmd.Root().GenBashCompletionV2(w, true)
	case "zsh":
		return cmd.Root().GenZshCompletion(w)
	case "fish":
		return cmd.Root().GenFishCompletion(w, true)
	default:
		return fmt.Errorf("unsupported shell %q: use bash, zsh or fish", shell)
	}
}
