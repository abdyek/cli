package project

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/confed"
	"github.com/umono-cms/cli/internal/download"
)

func Create(cmd *cobra.Command, projectPath, username, password string) error {
	client := download.NewClient()

	releaseInfo, err := client.GetLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch release: %w", err)
	}

	if err := client.DownloadAndExtract(releaseInfo, projectPath); err != nil {
		return err
	}

	envEditor := confed.NewEnvEditor()
	envEditor.Read(filepath.Join(projectPath, ".env.example"))
	envEditor.SetValue("APP_ENV", "prod")
	envEditor.SetValue("SESSION_DRIVER", "memory")
	envEditor.SetValue("USERNAME", username)
	envEditor.SetValue("PASSWORD", password)
	envEditor.Write(filepath.Join(projectPath, ".env"))

	// TODO: Add HASHED_USERNAME and HASHED_PASSWORD

	return nil
}
