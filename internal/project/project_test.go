package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/umono-cms/cli/internal/umonoproject"
)

func TestUpgradeRequiresAUmonoProject(t *testing.T) {
	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "umono-linux-amd64")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	err := Upgrade(dir)
	if !errors.Is(err, umonoproject.ErrNotUmonoProject) {
		t.Fatalf("err = %v, want ErrNotUmonoProject", err)
	}
}

func TestUpgradeLooksForTheBinaryAfterDetection(t *testing.T) {
	dir := t.TempDir()

	umonoJSONPath := filepath.Join(dir, "umono.json")
	if err := os.WriteFile(umonoJSONPath, []byte(`{"min_cli_version": "0.2.2"}`), 0o644); err != nil {
		t.Fatalf("write umono.json: %v", err)
	}

	err := Upgrade(dir)
	if err == nil {
		t.Fatalf("Upgrade returned nil error, want non-nil")
	}
	if errors.Is(err, umonoproject.ErrNotUmonoProject) {
		t.Fatalf("err = %v should not satisfy ErrNotUmonoProject", err)
	}
	if !strings.Contains(err.Error(), "binary") {
		t.Fatalf("err = %v, want it to contain %q", err, "binary")
	}
}

// writeFiles writes files, a map from a slash path to its content, under dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o755); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
}

// readFile returns the content of path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestApplyReleaseReplacesTheBinaryAndUmonoJSON(t *testing.T) {
	project := t.TempDir()
	release := t.TempDir()

	writeFiles(t, project, map[string]string{
		"umono":      "old binary",
		"umono.json": `{"min_cli_version": "0.2.2", "theme_api_current": 1, "theme_api_min": 1}`,
	})

	newManifest := `{"min_cli_version": "0.3.0", "theme_api_current": 2, "theme_api_min": 1}`
	writeFiles(t, release, map[string]string{
		"umono":      "new binary",
		"umono.json": newManifest,
	})

	if err := applyRelease(project, release); err != nil {
		t.Fatalf("applyRelease returned err = %v, want nil", err)
	}

	if got, want := readFile(t, filepath.Join(project, "umono")), "new binary"; got != want {
		t.Fatalf("umono content = %q, want %q", got, want)
	}
	if got, want := readFile(t, filepath.Join(project, "umono.json")), newManifest; got != want {
		t.Fatalf("umono.json content = %q, want %q", got, want)
	}
}

func TestApplyReleaseNeedsUmonoJSON(t *testing.T) {
	project := t.TempDir()
	release := t.TempDir()

	writeFiles(t, project, map[string]string{
		"umono":      "old binary",
		"umono.json": `{"min_cli_version": "0.2.2"}`,
	})

	writeFiles(t, release, map[string]string{
		"umono": "new binary",
	})

	err := applyRelease(project, release)
	if err == nil {
		t.Fatalf("applyRelease returned nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "umono.json") {
		t.Fatalf("err = %v, want it to contain %q", err, "umono.json")
	}

	if got, want := readFile(t, filepath.Join(project, "umono")), "old binary"; got != want {
		t.Fatalf("umono content = %q, want %q", got, want)
	}
	if got, want := readFile(t, filepath.Join(project, "umono.json")), `{"min_cli_version": "0.2.2"}`; got != want {
		t.Fatalf("umono.json content = %q, want %q", got, want)
	}
}

func TestApplyReleaseNeedsTheBinary(t *testing.T) {
	project := t.TempDir()
	release := t.TempDir()

	writeFiles(t, project, map[string]string{
		"umono":      "old binary",
		"umono.json": `{"min_cli_version": "0.2.2"}`,
	})

	writeFiles(t, release, map[string]string{
		"umono.json": `{"min_cli_version": "0.3.0"}`,
	})

	err := applyRelease(project, release)
	if err == nil {
		t.Fatalf("applyRelease returned nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "binary") {
		t.Fatalf("err = %v, want it to contain %q", err, "binary")
	}

	if got, want := readFile(t, filepath.Join(project, "umono")), "old binary"; got != want {
		t.Fatalf("umono content = %q, want %q", got, want)
	}
	if got, want := readFile(t, filepath.Join(project, "umono.json")), `{"min_cli_version": "0.2.2"}`; got != want {
		t.Fatalf("umono.json content = %q, want %q", got, want)
	}
}
