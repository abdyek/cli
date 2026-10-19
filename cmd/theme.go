package cmd

import (
	"github.com/spf13/cobra"
)

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Develop, build and install themes",
}

func init() {
	themeCmd.AddCommand(themeCreateCmd, themeDevCmd, themeBuildCmd, themeInstallCmd)
	rootCmd.AddCommand(themeCmd)
}
