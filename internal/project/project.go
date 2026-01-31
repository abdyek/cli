package project

import (
	"encoding/base64"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/confed"
	"github.com/umono-cms/cli/internal/download"
	"golang.org/x/crypto/bcrypt"
)

type Project struct {
	Username string
	Password string
	Path     string
	Port     string
}

func Create(cmd *cobra.Command, project Project) error {
	client := download.NewClient()

	releaseInfo, err := client.GetLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch release: %w", err)
	}

	if err := client.DownloadAndExtract(releaseInfo, project.Path); err != nil {
		return err
	}

	hashedUsername, err := hashData(project.Username)
	if err != nil {
		return fmt.Errorf("failed to hash Username: %w", err)
	}

	hashedPassword, err := hashData(project.Password)
	if err != nil {
		return fmt.Errorf("failed to hash Password: %w", err)
	}

	envEditor := confed.NewEnvEditor()
	envEditor.Read(filepath.Join(project.Path, ".env.example"))
	err = envEditor.SetValue("APP_ENV", "prod").
		SetValue("SESSION_DRIVER", "memory").
		AddBlankLine().
		SetValue("PORT", ":"+project.Port).
		SetValue("DSN", "umono.db").
		AddBlankLine().
		SetValue("USERNAME", "").
		SetValue("PASSWORD", "").
		AddBlankLine().
		SetValue("HASHED_USERNAME", base64.StdEncoding.EncodeToString([]byte(hashedUsername))).
		SetValue("HASHED_PASSWORD", base64.StdEncoding.EncodeToString([]byte(hashedPassword))).
		Write(filepath.Join(project.Path, ".env"))
	if err != nil {
		return fmt.Errorf("failed to write .env", err)
	}

	return nil
}

func hashData(data string) (string, error) {
	hashedData, err := bcrypt.GenerateFromPassword([]byte(data), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedData), nil
}
