package cmd

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/themeinstall"
)

var themeInstallCmd = &cobra.Command{
	Use:   "install <file>",
	Short: "Install a built theme into this Umono project",
	Long:  "Install a built theme (.umono-theme) into the Umono project in the working directory as themes/{org}/{id}/{version}/. Activate it in the admin panel.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		allow, _ := cmd.Flags().GetBool("allow-incompatible-api")
		return runThemeInstall(".", args[0], themeinstall.Options{Force: force, AllowIncompatibleAPI: allow}, cmd.OutOrStdout())
	},
}

func init() {
	themeInstallCmd.Flags().Bool("force", false, "Reinstall an installed version; the package waits in themes-pending/ for approval in the admin panel")
	themeInstallCmd.Flags().Bool("allow-incompatible-api", false, "Install even if the theme's Theme API version is outside this Umono's range")
}

// runThemeInstall installs file into the Umono project dir and tells what to
// do next.
func runThemeInstall(dir, file string, opts themeinstall.Options, out io.Writer) error {
	res, err := themeinstall.Install(dir, file, opts)
	if err != nil {
		return err
	}
	path := res.Dir
	if rel, err := filepath.Rel(dir, res.Dir); err == nil {
		path = rel
	}
	if res.Incompatible {
		fmt.Fprintf(out, "Warning: theme-api-version %d is outside the range of this Umono. The theme cannot be activated until Umono supports it.\n", res.Built.ThemeAPIVersion)
	}
	if res.Pending {
		fmt.Fprintf(out, "%s %s is already installed. The new package waits in %s.\nApprove the reinstall in the admin panel.\n", res.Built.ID, res.Built.Version, path)
		return nil
	}
	fmt.Fprintf(out, "Installed %s %s into %s.\nActivate it in the admin panel.\n", res.Built.ID, res.Built.Version, path)
	return nil
}
