package devserver

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/umono-cms/cli/internal/download"
	"github.com/umono-cms/cli/internal/download/downloadtest"
	"github.com/umono-cms/cli/internal/umonoproject"
	"github.com/umono-cms/themecore"
)

func releaseFiles(binary, min string) map[string][]byte {
	return map[string][]byte{
		"umono":        []byte(binary),
		".env.example": []byte("APP_ENV=\n"),
		"umono.json":   []byte(fmt.Sprintf(`{"min_cli_version": %q, "theme_api_current": %d, "theme_api_min": %d}`, min, themecore.ThemeAPICurrent(), themecore.ThemeAPIMin())),
	}
}

func fakeClient(t *testing.T, files map[string][]byte) *download.Client {
	t.Helper()
	srv := downloadtest.New(t, "v0.9.0", files)
	client, err := download.NewClientWithURL(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithURL: %v", err)
	}
	return client
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func writeUmonoJSON(t *testing.T, project, s string) {
	t.Helper()
	dir := Dir(project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "umono.json"), []byte(s), 0o644); err != nil {
		t.Fatalf("write umono.json: %v", err)
	}
}

func TestEnsureSetsTheServerUp(t *testing.T) {
	project := t.TempDir()
	created, err := Ensure(project, fakeClient(t, releaseFiles("binary", "0.0.0")))
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !created {
		t.Fatalf("created = false, want true")
	}

	if got, want := readFile(t, filepath.Join(Dir(project), "umono")), "binary"; got != want {
		t.Fatalf("umono content = %q, want %q", got, want)
	}
	env := readFile(t, filepath.Join(Dir(project), ".env"))
	if !strings.Contains(env, "HASHED_USERNAME=") {
		t.Fatalf(".env = %q, want it to contain %q", env, "HASHED_USERNAME=")
	}

	created, err = Ensure(project, nil)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if created {
		t.Fatalf("created = true, want false")
	}
}

func TestEnsureLeavesNothingOnFailure(t *testing.T) {
	project := t.TempDir()
	if _, err := Ensure(project, fakeClient(t, releaseFiles("binary", "99.0.0"))); err == nil {
		t.Fatalf("Ensure returned nil error, want non-nil")
	}

	devDir := filepath.Join(project, ".dev")
	entries, err := os.ReadDir(devDir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("os.ReadDir(%s): err = %v, want nil or fs.ErrNotExist", devDir, err)
		}
		return
	}
	if len(entries) != 0 {
		t.Fatalf("os.ReadDir(%s): %d entries, want 0", devDir, len(entries))
	}
}

func TestEnsureKeepsADirectoryThatIsNotAProject(t *testing.T) {
	project := t.TempDir()

	dbDir := Dir(project)
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dbDir, err)
	}
	dbPath := filepath.Join(dbDir, "umono.db")
	if err := os.WriteFile(dbPath, []byte("data"), 0o644); err != nil {
		t.Fatalf("write %s: %v", dbPath, err)
	}

	if _, err := Ensure(project, nil); err == nil {
		t.Fatalf("Ensure returned nil error, want non-nil")
	}

	if got, want := readFile(t, dbPath), "data"; got != want {
		t.Fatalf("umono.db content = %q, want %q", got, want)
	}
}

func TestCheckThemeAPI(t *testing.T) {
	cur, min := themecore.ThemeAPICurrent(), themecore.ThemeAPIMin()

	cases := []struct {
		name    string
		content string
	}{
		{
			"same range",
			fmt.Sprintf(`{"theme_api_current": %d, "theme_api_min": %d}`, cur, min),
		},
		{
			"runtime is newer",
			fmt.Sprintf(`{"theme_api_current": %d, "theme_api_min": %d}`, cur+1, min),
		},
		{
			"CLI is outdated",
			fmt.Sprintf(`{"theme_api_current": %d, "theme_api_min": %d}`, cur+1, cur+1),
		},
		{
			"no theme support",
			`{"min_cli_version": "0.0.0"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			project := t.TempDir()
			writeUmonoJSON(t, project, tc.content)

			warning, err := CheckThemeAPI(project)

			switch tc.name {
			case "same range":
				if err != nil {
					t.Fatalf("CheckThemeAPI: err = %v, want nil", err)
				}
				if warning != "" {
					t.Fatalf("warning = %q, want empty", warning)
				}
			case "runtime is newer":
				if err != nil {
					t.Fatalf("CheckThemeAPI: err = %v, want nil", err)
				}
				if !strings.Contains(warning, "--update-server") {
					t.Fatalf("warning = %q, want it to contain %q", warning, "--update-server")
				}
			case "CLI is outdated":
				if !errors.Is(err, ErrCLIOutdated) {
					t.Fatalf("err = %v, want errors.Is ErrCLIOutdated", err)
				}
			case "no theme support":
				if !errors.Is(err, umonoproject.ErrNoThemeSupport) {
					t.Fatalf("err = %v, want errors.Is umonoproject.ErrNoThemeSupport", err)
				}
			}
		})
	}
}

func TestUpdateKeepsTheDatabase(t *testing.T) {
	project := t.TempDir()
	if _, err := Ensure(project, fakeClient(t, releaseFiles("old", "0.0.0"))); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	dbPath := filepath.Join(Dir(project), "umono.db")
	if err := os.WriteFile(dbPath, []byte("data"), 0o644); err != nil {
		t.Fatalf("write %s: %v", dbPath, err)
	}

	if err := Update(project, fakeClient(t, releaseFiles("new", "0.0.0"))); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if got, want := readFile(t, filepath.Join(Dir(project), "umono")), "new"; got != want {
		t.Fatalf("umono content = %q, want %q", got, want)
	}
	if got, want := readFile(t, dbPath), "data"; got != want {
		t.Fatalf("umono.db content = %q, want %q", got, want)
	}
}

func TestUpdateWithoutAServer(t *testing.T) {
	err := Update(t.TempDir(), fakeClient(t, releaseFiles("new", "0.0.0")))
	if !errors.Is(err, umonoproject.ErrNotUmonoProject) {
		t.Fatalf("err = %v, want errors.Is umonoproject.ErrNotUmonoProject", err)
	}
}
