package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/umono-cms/cli/internal/download"
	"github.com/umono-cms/cli/internal/download/downloadtest"
	"github.com/umono-cms/cli/internal/umonoproject"
)

const builtTemplate = `schema = 'https://umono.dev/schemas/theme-built/v1'
id = '%s'
contract = 'example/blog@v1.0'
theme-api-version = %d
name = 'Test'
version = '%s'
built-at = 2026-10-14T00:00:00Z
`

// builtTOML returns a valid built.toml of the theme id at version with the
// Theme API version api.
func builtTOML(id, version string, api int) string {
	return fmt.Sprintf(builtTemplate, id, api, version)
}

func TestUpgradeRequiresAUmonoProject(t *testing.T) {
	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "umono-linux-amd64")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	err := Upgrade(dir, false)
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

	err := Upgrade(dir, false)
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

	if err := applyRelease(project, release, false, io.Discard); err != nil {
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

	err := applyRelease(project, release, false, io.Discard)
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

	err := applyRelease(project, release, false, io.Discard)
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

func TestApplyReleaseStopsOnAThemeOutsideTheRange(t *testing.T) {
	project := t.TempDir()
	release := t.TempDir()

	oldManifest := `{"min_cli_version": "0.2.2", "theme_api_current": 1, "theme_api_min": 1}`
	writeFiles(t, project, map[string]string{
		"umono":                                "old binary",
		"umono.json":                           oldManifest,
		"themes/acme/minimal/0.1.0/built.toml": builtTOML("acme/minimal", "0.1.0", 1),
	})

	writeFiles(t, release, map[string]string{
		"umono":      "new binary",
		"umono.json": `{"min_cli_version": "0.3.0", "theme_api_current": 2, "theme_api_min": 2}`,
	})

	err := applyRelease(project, release, false, io.Discard)
	if err == nil {
		t.Fatalf("applyRelease returned nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "acme/minimal 0.1.0") {
		t.Fatalf("err = %v, want it to contain %q", err, "acme/minimal 0.1.0")
	}
	if !strings.Contains(err.Error(), "Theme API 1") {
		t.Fatalf("err = %v, want it to contain %q", err, "Theme API 1")
	}
	if !strings.Contains(err.Error(), "--allow-theme-break") {
		t.Fatalf("err = %v, want it to contain %q", err, "--allow-theme-break")
	}

	if got, want := readFile(t, filepath.Join(project, "umono")), "old binary"; got != want {
		t.Fatalf("umono content = %q, want %q", got, want)
	}
	if got, want := readFile(t, filepath.Join(project, "umono.json")), oldManifest; got != want {
		t.Fatalf("umono.json content = %q, want %q", got, want)
	}
}

func TestApplyReleaseAllowsAThemeBreak(t *testing.T) {
	project := t.TempDir()
	release := t.TempDir()

	writeFiles(t, project, map[string]string{
		"umono":                                "old binary",
		"umono.json":                           `{"min_cli_version": "0.2.2", "theme_api_current": 1, "theme_api_min": 1}`,
		"themes/acme/minimal/0.1.0/built.toml": builtTOML("acme/minimal", "0.1.0", 1),
	})

	writeFiles(t, release, map[string]string{
		"umono":      "new binary",
		"umono.json": `{"min_cli_version": "0.3.0", "theme_api_current": 2, "theme_api_min": 2}`,
	})

	var out bytes.Buffer
	err := applyRelease(project, release, true, &out)
	if err != nil {
		t.Fatalf("applyRelease returned err = %v, want nil", err)
	}

	if got, want := readFile(t, filepath.Join(project, "umono")), "new binary"; got != want {
		t.Fatalf("umono content = %q, want %q", got, want)
	}
	if !strings.Contains(out.String(), "acme/minimal 0.1.0") {
		t.Fatalf("out = %q, want it to contain %q", out.String(), "acme/minimal 0.1.0")
	}
}

func TestApplyReleaseKeepsAThemeInTheRange(t *testing.T) {
	project := t.TempDir()
	release := t.TempDir()

	writeFiles(t, project, map[string]string{
		"umono":                                "old binary",
		"umono.json":                           `{"min_cli_version": "0.2.2", "theme_api_current": 1, "theme_api_min": 1}`,
		"themes/acme/minimal/0.1.0/built.toml": builtTOML("acme/minimal", "0.1.0", 1),
	})

	writeFiles(t, release, map[string]string{
		"umono":      "new binary",
		"umono.json": `{"min_cli_version": "0.3.0", "theme_api_current": 2, "theme_api_min": 1}`,
	})

	err := applyRelease(project, release, false, io.Discard)
	if err != nil {
		t.Fatalf("applyRelease returned err = %v, want nil", err)
	}
	if got, want := readFile(t, filepath.Join(project, "umono")), "new binary"; got != want {
		t.Fatalf("umono content = %q, want %q", got, want)
	}
}

func TestApplyReleaseStopsOnAnUnreadableTheme(t *testing.T) {
	project := t.TempDir()
	release := t.TempDir()

	oldManifest := `{"min_cli_version": "0.2.2", "theme_api_current": 1, "theme_api_min": 1}`
	writeFiles(t, project, map[string]string{
		"umono":                               "old binary",
		"umono.json":                          oldManifest,
		"themes/acme/broken/0.1.0/built.toml": "not a built.toml",
	})

	writeFiles(t, release, map[string]string{
		"umono":      "new binary",
		"umono.json": `{"min_cli_version": "0.3.0", "theme_api_current": 2, "theme_api_min": 1}`,
	})

	err := applyRelease(project, release, false, io.Discard)
	if err == nil {
		t.Fatalf("applyRelease returned nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "acme/broken/0.1.0") {
		t.Fatalf("err = %v, want it to contain %q", err, "acme/broken/0.1.0")
	}

	if got, want := readFile(t, filepath.Join(project, "umono")), "old binary"; got != want {
		t.Fatalf("umono content = %q, want %q", got, want)
	}
	if got, want := readFile(t, filepath.Join(project, "umono.json")), oldManifest; got != want {
		t.Fatalf("umono.json content = %q, want %q", got, want)
	}
}

func TestApplyReleaseSkipsTheDevTheme(t *testing.T) {
	project := t.TempDir()
	release := t.TempDir()

	writeFiles(t, project, map[string]string{
		"umono":                           "old binary",
		"umono.json":                      `{"min_cli_version": "0.2.2", "theme_api_current": 1, "theme_api_min": 1}`,
		"themes/dev/dev/0.0.0/built.toml": "not a built.toml",
	})

	writeFiles(t, release, map[string]string{
		"umono":      "new binary",
		"umono.json": `{"min_cli_version": "0.3.0", "theme_api_current": 2, "theme_api_min": 1}`,
	})

	err := applyRelease(project, release, false, io.Discard)
	if err != nil {
		t.Fatalf("applyRelease returned err = %v, want nil", err)
	}
}

func TestApplyReleaseStopsWhenTheReleaseHasNoThemes(t *testing.T) {
	project := t.TempDir()
	release := t.TempDir()

	oldManifest := `{"min_cli_version": "0.2.2", "theme_api_current": 1, "theme_api_min": 1}`
	writeFiles(t, project, map[string]string{
		"umono":                                "old binary",
		"umono.json":                           oldManifest,
		"themes/acme/minimal/0.1.0/built.toml": builtTOML("acme/minimal", "0.1.0", 1),
	})

	writeFiles(t, release, map[string]string{
		"umono":      "new binary",
		"umono.json": `{"min_cli_version": "0.3.0"}`,
	})

	err := applyRelease(project, release, false, io.Discard)
	if err == nil {
		t.Fatalf("applyRelease returned nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "acme/minimal 0.1.0") {
		t.Fatalf("err = %v, want it to contain %q", err, "acme/minimal 0.1.0")
	}
	if !strings.Contains(err.Error(), "does not support themes") {
		t.Fatalf("err = %v, want it to contain %q", err, "does not support themes")
	}

	if got, want := readFile(t, filepath.Join(project, "umono")), "old binary"; got != want {
		t.Fatalf("umono content = %q, want %q", got, want)
	}
	if got, want := readFile(t, filepath.Join(project, "umono.json")), oldManifest; got != want {
		t.Fatalf("umono.json content = %q, want %q", got, want)
	}
}

// releaseFiles are the files of a fake Umono release with min_cli_version
// min.
func releaseFiles(min string) map[string][]byte {
	return map[string][]byte{
		"umono":        []byte("new binary"),
		"umono.json":   []byte(`{"min_cli_version": "` + min + `", "theme_api_current": 1, "theme_api_min": 1}`),
		".env.example": []byte("APP_ENV=\n"),
	}
}

func TestCreateWithDownloadsTheLatestRelease(t *testing.T) {
	srv := downloadtest.New(t, "v0.9.0", releaseFiles("0.0.0"))
	client, err := download.NewClientWithURL(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithURL: %v", err)
	}
	dir := t.TempDir()
	err = CreateWith(client, Project{Username: "admin", Password: "admin", Path: dir, Port: "8999"})
	if err != nil {
		t.Fatalf("CreateWith: %v", err)
	}
	if got, want := readFile(t, filepath.Join(dir, "umono")), "new binary"; got != want {
		t.Fatalf("umono content = %q, want %q", got, want)
	}
	env := readFile(t, filepath.Join(dir, ".env"))
	if !strings.Contains(env, "HASHED_USERNAME=") {
		t.Fatalf(".env = %q, want it to contain %q", env, "HASHED_USERNAME=")
	}
	if !strings.Contains(env, "PORT=8999") {
		t.Fatalf(".env = %q, want it to contain %q", env, "PORT=8999")
	}
}

func TestCreateWithStopsOnAnOldCLI(t *testing.T) {
	srv := downloadtest.New(t, "v0.9.0", releaseFiles("99.0.0"))
	client, err := download.NewClientWithURL(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithURL: %v", err)
	}
	dir := t.TempDir()
	err = CreateWith(client, Project{Username: "admin", Password: "admin", Path: dir, Port: "8999"})
	if err == nil {
		t.Fatalf("CreateWith returned nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "CLI version incompatible") {
		t.Fatalf("err = %v, want it to contain %q", err, "CLI version incompatible")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "umono")); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("os.Stat umono: err = %v, want fs.ErrNotExist", statErr)
	}
}

func TestUpgradeWithReplacesTheBinaryAndUmonoJSON(t *testing.T) {
	srv := downloadtest.New(t, "v0.9.0", releaseFiles("0.0.0"))
	client, err := download.NewClientWithURL(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithURL: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "umono"), []byte("old binary"), 0o755); err != nil {
		t.Fatalf("write umono: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "umono.json"), []byte(`{"min_cli_version": "0.0.0", "theme_api_current": 1, "theme_api_min": 1}`), 0o644); err != nil {
		t.Fatalf("write umono.json: %v", err)
	}
	if err := UpgradeWith(client, dir, false); err != nil {
		t.Fatalf("UpgradeWith: %v", err)
	}
	if got, want := readFile(t, filepath.Join(dir, "umono")), "new binary"; got != want {
		t.Fatalf("umono content = %q, want %q", got, want)
	}
	if got, want := readFile(t, filepath.Join(dir, "umono.json")), string(releaseFiles("0.0.0")["umono.json"]); got != want {
		t.Fatalf("umono.json content = %q, want %q", got, want)
	}
}
