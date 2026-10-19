package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/download"
	"github.com/umono-cms/cli/internal/themedev"
	"github.com/umono-cms/cli/internal/themeproject"
)

var (
	themeDevPort         int
	themeDevUpdateServer bool
)

var themeDevCmd = &cobra.Command{
	Use:   "dev",
	Short: "Develop the theme with a live Umono",
	Long: `Run the theme project in the working directory on the Umono of .dev/server,
behind the dev proxy at http://localhost:8990, with live reload and an error
overlay.

The first run sets .dev/server up with the latest Umono release and the
account admin/admin. It is never updated by itself: --update-server updates
it to the latest release and keeps its database.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return themedev.Run(ctx, themedev.Options{
			ProjectDir:   ".",
			Port:         themeDevPort,
			UpdateServer: themeDevUpdateServer,
			Client:       download.NewClient(),
			Registry:     themeproject.DefaultRegistry,
			Out:          cmd.OutOrStdout(),
			Interval:     300 * time.Millisecond,
		})
	},
}

func init() {
	themeDevCmd.Flags().IntVar(&themeDevPort, "port", themedev.DefaultPort, "Port of the dev proxy")
	themeDevCmd.Flags().BoolVar(&themeDevUpdateServer, "update-server", false, "Update .dev/server to the latest Umono release first, keeping its database")
}
