package devtheme

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/umono-cms/cli/internal/themeproject"
	"github.com/umono-cms/themecore"
)

var now = time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "themeproject", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newProject(t *testing.T) (string, string) {
	t.Helper()
	projectDir := filepath.Join(t.TempDir(), "theme")
	writeFile(t, filepath.Join(projectDir, "theme.toml"), fixture(t, "minimal.toml"))
	writeFile(t, filepath.Join(projectDir, "assets", "theme.css"), themecore.FallbackSource())
	writeFile(t, filepath.Join(projectDir, ".dev", "contracts", "umono", "page", "v0.1.toml"), fixture(t, "page-v0.1.toml"))
	serverDir := filepath.Join(projectDir, ".dev", "server")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", serverDir, err)
	}
	return projectDir, serverDir
}

var noRegistry = themeproject.Registry{BaseURL: "http://127.0.0.1:1", Client: &http.Client{Timeout: time.Second}}

func link(t *testing.T, serverDir, name string) string {
	t.Helper()
	target, err := os.Readlink(filepath.Join(serverDir, Path, name))
	if err != nil {
		t.Fatalf("readlink %s: %v", name, err)
	}
	return target
}

func TestSyncWritesTheDevDirectory(t *testing.T) {
	projectDir, serverDir := newProject(t)

	state, err := Sync(context.Background(), projectDir, serverDir, noRegistry, now)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if state.Err != nil {
		t.Fatalf("state.Err = %v, want nil", state.Err)
	}
	if state.Contract != "umono/page@v0.1" {
		t.Fatalf("state.Contract = %q, want %q", state.Contract, "umono/page@v0.1")
	}

	if got, want := link(t, serverDir, "assets"), filepath.Join(projectDir, "assets"); got != want {
		t.Fatalf("assets link = %q, want %q", got, want)
	}
	if got, want := link(t, serverDir, "contract.toml"), filepath.Join(projectDir, ".dev", "contracts", "umono", "page", "v0.1.toml"); got != want {
		t.Fatalf("contract.toml link = %q, want %q", got, want)
	}

	if _, err := os.Lstat(filepath.Join(serverDir, Path, "attributes.toml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lstat attributes.toml err = %v, want os.ErrNotExist", err)
	}

	b, err := os.ReadFile(filepath.Join(serverDir, Path, "built.toml"))
	if err != nil {
		t.Fatalf("read built.toml: %v", err)
	}
	if _, err := themecore.ParseBuilt(b); err == nil {
		t.Fatalf("ParseBuilt succeeded; want error for reserved dev/dev")
	}
	text := string(b)
	for _, want := range []string{
		"id = \"dev/dev\"",
		"version = \"0.0.0\"",
		"contract = \"umono/page@v0.1\"",
		"theme-api-version = 1",
		"built-at = 2026-06-02T00:00:00Z",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("built.toml missing %q", want)
		}
	}
}

func TestSyncLinksAttributesAndRemovesTheLink(t *testing.T) {
	projectDir, serverDir := newProject(t)

	writeFile(t, filepath.Join(projectDir, "attributes.toml"), []byte("schema = \"https://umono.dev/schemas/theme-attributes/v1\"\n"))

	state, err := Sync(context.Background(), projectDir, serverDir, noRegistry, now)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if state.Err != nil {
		t.Fatalf("state.Err = %v, want nil", state.Err)
	}
	if got, want := link(t, serverDir, "attributes.toml"), filepath.Join(projectDir, "attributes.toml"); got != want {
		t.Fatalf("attributes.toml link = %q, want %q", got, want)
	}

	if err := os.Remove(filepath.Join(projectDir, "attributes.toml")); err != nil {
		t.Fatalf("remove attributes.toml: %v", err)
	}
	if _, err := Sync(context.Background(), projectDir, serverDir, noRegistry, now); err != nil {
		t.Fatalf("Sync after remove: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(serverDir, Path, "attributes.toml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lstat attributes.toml err = %v, want os.ErrNotExist", err)
	}
}

func TestSyncReportsAFailedCheck(t *testing.T) {
	projectDir, serverDir := newProject(t)

	writeFile(t, filepath.Join(projectDir, "assets", "theme.css"), []byte("html { color: red }"))

	state, err := Sync(context.Background(), projectDir, serverDir, noRegistry, now)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if state.Err == nil {
		t.Fatalf("state.Err = nil, want non-nil")
	}

	b, err := os.ReadFile(filepath.Join(serverDir, Path, "built.toml"))
	if err != nil {
		t.Fatalf("read built.toml: %v", err)
	}
	want := "theme-api-version = " + strconv.Itoa(themecore.ThemeAPICurrent())
	if !strings.Contains(string(b), want) {
		t.Fatalf("built.toml missing %q", want)
	}
}

func TestSyncFetchesAMissingContract(t *testing.T) {
	projectDir, serverDir := newProject(t)

	if err := os.RemoveAll(filepath.Join(projectDir, ".dev", "contracts")); err != nil {
		t.Fatalf("remove contracts: %v", err)
	}

	body := fixture(t, "page-v0.1.toml")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/registry/v1/contracts/umono/page/v0.1" {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	defer srv.Close()

	state, err := Sync(context.Background(), projectDir, serverDir, themeproject.Registry{BaseURL: srv.URL, Client: srv.Client()}, now)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if state.Err != nil {
		t.Fatalf("state.Err = %v, want nil", state.Err)
	}

	got, err := os.ReadFile(filepath.Join(projectDir, ".dev", "contracts", "umono", "page", "v0.1.toml"))
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("contract bytes differ")
	}
}

func TestSyncFailsWithoutTheContract(t *testing.T) {
	projectDir, serverDir := newProject(t)

	if err := os.RemoveAll(filepath.Join(projectDir, ".dev", "contracts")); err != nil {
		t.Fatalf("remove contracts: %v", err)
	}

	_, err := Sync(context.Background(), projectDir, serverDir, noRegistry, now)
	if !errors.Is(err, themeproject.ErrRegistryUnreachable) {
		t.Fatalf("err = %v, want themeproject.ErrRegistryUnreachable", err)
	}
}

func TestSyncFailsWithoutThemeTOML(t *testing.T) {
	projectDir, serverDir := newProject(t)

	if err := os.Remove(filepath.Join(projectDir, "theme.toml")); err != nil {
		t.Fatalf("remove theme.toml: %v", err)
	}

	_, err := Sync(context.Background(), projectDir, serverDir, noRegistry, now)
	if err == nil {
		t.Fatalf("err = nil, want non-nil")
	}
}
