package project

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/compatibility"
	"github.com/umono-cms/cli/internal/confed"
	"github.com/umono-cms/cli/internal/download"
	"github.com/umono-cms/cli/internal/umonoproject"
	crypto "github.com/umono-cms/crypto"
	"golang.org/x/crypto/bcrypt"
)

type Project struct {
	Username string
	Password string
	Path     string
	Port     string
}

func Create(cmd *cobra.Command, project Project) error {
	return CreateWith(download.NewClient(), project)
}

// CreateWith creates project with the latest release of client.
func CreateWith(client *download.Client, project Project) error {
	result, err := compatibility.Check(client)
	if err != nil {
		return fmt.Errorf("failed to check compatibility: %w", err)
	}

	if !result.Compatible {
		return fmt.Errorf("%s", compatibility.FormatIncompatibleError(result))
	}

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

	secretKey, err := crypto.GenerateKey()
	if err != nil {
		return fmt.Errorf("failed to generate UMONO_SECRET: %w", err)
	}

	envEditor := confed.NewEnvEditor()
	if err := envEditor.Read(filepath.Join(project.Path, ".env.example")); err != nil {
		return fmt.Errorf("failed to read .env.example: %w", err)
	}

	err = envEditor.PrependValue("UMONO_SECRET", secretKey.String()).
		SetValue("APP_ENV", "prod").
		SetValue("SESSION_DRIVER", "memory").
		AddBlankLine().
		SetValue("PORT", project.Port).
		SetValue("DSN", "umono.db").
		AddBlankLine().
		SetValue("USERNAME", "").
		SetValue("PASSWORD", "").
		AddBlankLine().
		SetValue("HASHED_USERNAME", base64.StdEncoding.EncodeToString([]byte(hashedUsername))).
		SetValue("HASHED_PASSWORD", base64.StdEncoding.EncodeToString([]byte(hashedPassword))).
		Write(filepath.Join(project.Path, ".env"))
	if err != nil {
		return fmt.Errorf("failed to write .env: %w", err)
	}

	return nil
}

func Upgrade(projectPath string, allowThemeBreak bool) error {
	return UpgradeWith(download.NewClient(), projectPath, allowThemeBreak)
}

// UpgradeWith upgrades the project at projectPath to the latest release of
// client.
func UpgradeWith(client *download.Client, projectPath string, allowThemeBreak bool) error {
	if _, err := umonoproject.Detect(projectPath); err != nil {
		return err
	}
	binaryPath := findBinaryPath(projectPath)
	if binaryPath == "" {
		return fmt.Errorf("no Umono binary found in %s", projectPath)
	}

	result, err := compatibility.Check(client)
	if err != nil {
		return fmt.Errorf("failed to check compatibility: %w", err)
	}

	if !result.Compatible {
		return fmt.Errorf("%s", compatibility.FormatIncompatibleError(result))
	}

	releaseInfo, err := client.GetLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch release: %w", err)
	}

	fmt.Printf("📦 Latest version: %s\n", releaseInfo.Version)

	tmpDir, err := os.MkdirTemp("", "umono-upgrade-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := client.DownloadAndExtract(releaseInfo, tmpDir); err != nil {
		return err
	}

	return applyRelease(projectPath, tmpDir, allowThemeBreak, os.Stdout)
}

// applyRelease installs the release extracted to releaseDir into projectDir:
// the binary and umono.json, which carries the release's min_cli_version and
// Theme API range. Nothing changes when the release lacks either. If
// umono.json cannot be replaced, the old binary is put back. The installed
// themes are checked against the release's Theme API range first: a theme
// that the release would not activate stops it, unless allowThemeBreak is
// set, which only reports it to out.
func applyRelease(projectDir, releaseDir string, allowThemeBreak bool, out io.Writer) error {
	binaryPath := findBinaryPath(projectDir)
	if binaryPath == "" {
		return fmt.Errorf("no Umono binary found in %s", projectDir)
	}
	newBinaryPath := findBinaryPath(releaseDir)
	if newBinaryPath == "" {
		return fmt.Errorf("no binary found in downloaded release")
	}
	if _, err := umonoproject.Detect(releaseDir); err != nil {
		return fmt.Errorf("downloaded release has no valid umono.json: %w", err)
	}

	breaks, err := checkThemes(projectDir, releaseDir)
	if err != nil {
		return fmt.Errorf("failed to check the installed themes: %w", err)
	}
	if len(breaks) > 0 {
		var list strings.Builder
		for _, b := range breaks {
			fmt.Fprintf(&list, "  - %s: %s\n", b.Theme, b.Reason)
		}
		if !allowThemeBreak {
			return fmt.Errorf("these installed themes would not work with the new Umono:\n%sUpgrade anyway with --allow-theme-break.", list.String())
		}
		fmt.Fprintf(out, "⚠️  These installed themes will not work with the new Umono:\n%s", list.String())
	}

	manifestPath := filepath.Join(projectDir, "umono.json")
	newManifestPath := manifestPath + ".new"
	if err := copyFile(filepath.Join(releaseDir, "umono.json"), newManifestPath); err != nil {
		os.Remove(newManifestPath)
		return fmt.Errorf("failed to prepare umono.json: %w", err)
	}

	backupPath := binaryPath + ".backup"
	if err := os.Rename(binaryPath, backupPath); err != nil {
		os.Remove(newManifestPath)
		return fmt.Errorf("failed to backup existing binary: %w", err)
	}

	if err := copyFile(newBinaryPath, binaryPath); err != nil {
		os.Rename(backupPath, binaryPath)
		os.Remove(newManifestPath)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	if err := os.Rename(newManifestPath, manifestPath); err != nil {
		os.Rename(backupPath, binaryPath)
		os.Remove(newManifestPath)
		return fmt.Errorf("failed to install umono.json: %w", err)
	}

	os.Remove(backupPath)
	return nil
}

func findBinaryPath(dir string) string {
	candidates := []string{"umono", "umono-darwin-amd64", "umono-darwin-arm64", "umono-linux-amd64", "umono-linux-arm64"}

	for _, name := range candidates {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}

	return ""
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	destFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func hashData(data string) (string, error) {
	hashedData, err := bcrypt.GenerateFromPassword([]byte(data), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedData), nil
}
