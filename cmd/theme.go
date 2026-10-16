package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

// ErrNotImplemented is returned by a theme command that is not written yet.
var ErrNotImplemented = errors.New("not implemented yet")

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Develop, build and install themes",
}

var themeDevCmd = &cobra.Command{
	Use:   "dev",
	Short: "Develop the theme with a live Umono",
	RunE:  notImplemented,
}

var themeBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the theme into dist/",
	RunE:  notImplemented,
}

var themeInstallCmd = &cobra.Command{
	Use:   "install <file>",
	Short: "Install a built theme into this Umono project",
	RunE:  notImplemented,
}

func init() {
	themeCmd.AddCommand(themeCreateCmd, themeDevCmd, themeBuildCmd, themeInstallCmd)
	rootCmd.AddCommand(themeCmd)
}

func notImplemented(cmd *cobra.Command, args []string) error {
	return ErrNotImplemented
}
