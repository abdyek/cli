package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var detach bool

var upCmd = &cobra.Command{
	Use:   "up <name>",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("up called")
	},
}

func init() {
  upCmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run in background")
	rootCmd.AddCommand(upCmd)
}
