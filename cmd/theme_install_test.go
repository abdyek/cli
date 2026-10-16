package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/umono-cms/cli/internal/themeinstall"
	"github.com/umono-cms/cli/internal/themeproject"
)

const installJSON = `{"min_cli_version": "0.3.0", "theme_api_current": 1, "theme_api_min": 1}`

func installProject(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "umono.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write umono.json: %v", err)
	}
	t.Cleanup(func() {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || !info.IsDir() {
				return nil
			}
			_ = os.Chmod(path, 0o755)
			return nil
		})
	})
	return dir
}

func builtThemeFile(t *testing.T) string {
	t.Helper()
	reg := pageContractRegistry(t)
	res, err := themeproject.Build(context.Background(), newThemeProject(t), reg, buildNow)
	if err != nil {
		t.Fatalf("themeproject.Build: %v", err)
	}
	return res.Path
}

func TestThemeInstall(t *testing.T) {
	file := builtThemeFile(t)
	proj := installProject(t, installJSON)

	var out bytes.Buffer
	if err := runThemeInstall(proj, file, themeinstall.Options{}, &out); err != nil {
		t.Fatalf("runThemeInstall: %v", err)
	}

	output := out.String()
	for _, want := range []string{"acme/minimal", "0.1.0", filepath.Join("themes", "acme", "minimal", "0.1.0")} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q does not contain %q", output, want)
		}
	}
	if strings.Contains(output, "themes-pending") {
		t.Fatalf("output %q should not contain %q", output, "themes-pending")
	}
}

func TestThemeInstallForce(t *testing.T) {
	file := builtThemeFile(t)
	proj := installProject(t, installJSON)

	var out bytes.Buffer
	if err := runThemeInstall(proj, file, themeinstall.Options{}, &out); err != nil {
		t.Fatalf("first runThemeInstall: %v", err)
	}
	out.Reset()

	if err := runThemeInstall(proj, file, themeinstall.Options{Force: true}, &out); err != nil {
		t.Fatalf("runThemeInstall (force): %v", err)
	}

	output := out.String()
	for _, want := range []string{filepath.Join("themes-pending", "acme", "minimal", "0.1.0"), "admin"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q does not contain %q", output, want)
		}
	}
}

func TestThemeInstallAlreadyInstalled(t *testing.T) {
	file := builtThemeFile(t)
	proj := installProject(t, installJSON)

	var out bytes.Buffer
	if err := runThemeInstall(proj, file, themeinstall.Options{}, &out); err != nil {
		t.Fatalf("first runThemeInstall: %v", err)
	}
	out.Reset()

	err := runThemeInstall(proj, file, themeinstall.Options{}, &out)
	if !errors.Is(err, themeinstall.ErrAlreadyInstalled) {
		t.Fatalf("second runThemeInstall err = %v, want ErrAlreadyInstalled", err)
	}
}

func TestThemeInstallIncompatibleWarning(t *testing.T) {
	file := builtThemeFile(t)
	proj := installProject(t, `{"min_cli_version": "0.3.0", "theme_api_current": 2, "theme_api_min": 2}`)

	var out bytes.Buffer
	if err := runThemeInstall(proj, file, themeinstall.Options{AllowIncompatibleAPI: true}, &out); err != nil {
		t.Fatalf("runThemeInstall: %v", err)
	}

	if !strings.Contains(strings.ToLower(out.String()), "warning") {
		t.Fatalf("output %q does not contain %q", strings.ToLower(out.String()), "warning")
	}
}

func TestThemeInstallFlags(t *testing.T) {
	if themeInstallCmd.Flags().Lookup("force") == nil {
		t.Fatalf("themeInstallCmd.Flags().Lookup(\"force\") is nil")
	}
	if themeInstallCmd.Flags().Lookup("allow-incompatible-api") == nil {
		t.Fatalf("themeInstallCmd.Flags().Lookup(\"allow-incompatible-api\") is nil")
	}
}
