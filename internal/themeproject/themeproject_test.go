package themeproject

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/umono-cms/themecore"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata %s: %v", name, err)
	}
	return b
}

var (
	full = Manifest{
		ID:          "acme/minimal",
		Contract:    "umono/blog@v0.1",
		Name:        "Minimal Blog",
		Version:     "0.1.0",
		Description: "Clean blog theme for Umono.",
		License:     "MIT",
		Homepage:    "https://example.com/minimal",
		Repository:  "https://example.com/minimal.git",
		AuthorName:  "Acme",
		AuthorURL:   "https://example.com",
	}
	minimal = Manifest{
		ID:       "acme/minimal",
		Name:     "Minimal",
		Contract: "umono/page@v0.1",
		Version:  "0.1.0",
	}
)

func TestRenderFull(t *testing.T) {
	if got := string(Render(full)); got != string(readTestdata(t, "full.toml")) {
		t.Fatalf("Render(full) mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, readTestdata(t, "full.toml"))
	}
	if err := full.Validate(); err != nil {
		t.Fatalf("full.Validate() = %v, want nil", err)
	}
}

func TestRenderMinimal(t *testing.T) {
	if got := string(Render(minimal)); got != string(readTestdata(t, "minimal.toml")) {
		t.Fatalf("Render(minimal) mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, readTestdata(t, "minimal.toml"))
	}
	if err := minimal.Validate(); err != nil {
		t.Fatalf("minimal.Validate() = %v, want nil", err)
	}
}

func TestRenderQuotes(t *testing.T) {
	m := minimal
	m.Name = "Say " + string(rune(34)) + "hi" + string(rune(34)) + " " + string(rune(92)) + " <ok> & ünicode"
	if got := string(Render(m)); got != string(readTestdata(t, "quoted.toml")) {
		t.Fatalf("Render(m) mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, readTestdata(t, "quoted.toml"))
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("m.Validate() = %v, want nil", err)
	}
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*Manifest)
	}{
		{"id reserved", func(m *Manifest) { m.ID = "dev/dev" }},
		{"id uppercase", func(m *Manifest) { m.ID = "Acme/x" }},
		{"contract missing v", func(m *Manifest) { m.Contract = "umono/page@0.1" }},
		{"version with v", func(m *Manifest) { m.Version = "v0.1.0" }},
		{"empty name", func(m *Manifest) { m.Name = "" }},
		{"author url without name", func(m *Manifest) { m.AuthorName = ""; m.AuthorURL = "https://example.com" }},
		{"bad license", func(m *Manifest) { m.License = "MIT OR" }},
		{"homepage http", func(m *Manifest) { m.Homepage = "http://example.com" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := minimal
			tc.mut(&m)
			if err := m.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
		})
	}
}

func TestCreate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-theme")
	if err := Create(dir, minimal); err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	sort.Strings(paths)
	wantPaths := []string{".gitignore", "assets/theme.css", "theme.toml"}
	if len(paths) != len(wantPaths) {
		t.Fatalf("paths = %v, want %v", paths, wantPaths)
	}
	for i, p := range paths {
		if p != wantPaths[i] {
			t.Fatalf("paths = %v, want %v", paths, wantPaths)
		}
	}

	gotTOML, err := os.ReadFile(filepath.Join(dir, "theme.toml"))
	if err != nil {
		t.Fatalf("read theme.toml: %v", err)
	}
	if string(gotTOML) != string(readTestdata(t, "minimal.toml")) {
		t.Fatalf("theme.toml mismatch:\n--- got ---\n%s\n--- want ---\n%s", gotTOML, readTestdata(t, "minimal.toml"))
	}

	gotGI, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if want := ".dev/\ndist/\n"; string(gotGI) != want {
		t.Fatalf(".gitignore = %q, want %q", gotGI, want)
	}

	gotCSS, err := os.ReadFile(filepath.Join(dir, "assets", "theme.css"))
	if err != nil {
		t.Fatalf("read assets/theme.css: %v", err)
	}
	if !strings.HasPrefix(string(gotCSS), "/*") {
		t.Fatalf("assets/theme.css does not start with %q:\n%s", "/*", gotCSS)
	}
	if !strings.HasSuffix(string(gotCSS), string(themecore.FallbackSource())) {
		t.Fatalf("assets/theme.css does not end with themecore.FallbackSource()")
	}
}

func TestCreateRefusesAnExistingPath(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "new-theme")
		if err := os.WriteFile(dir, []byte("keep"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := Create(dir, minimal); !errors.Is(err, ErrExists) {
			t.Fatalf("Create() = %v, want ErrExists", err)
		}
		got, err := os.ReadFile(dir)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(got) != "keep" {
			t.Fatalf("file contents = %q, want %q", got, "keep")
		}
	})
	t.Run("directory", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "new-theme")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("Mkdir: %v", err)
		}
		if err := Create(dir, minimal); !errors.Is(err, ErrExists) {
			t.Fatalf("Create() = %v, want ErrExists", err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("directory not empty: %d entries", len(entries))
		}
	})
}

func TestCreateValidatesFirst(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-theme")
	m := minimal
	m.ID = "dev/dev"
	if err := Create(dir, m); err == nil {
		t.Fatalf("Create() = nil, want error")
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(%s) err = %v, want fs.ErrNotExist", dir, err)
	}
}

func TestEnsureContract(t *testing.T) {
	count := 0
	body := "first"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if r.URL.Path != "/registry/v1/contracts/umono/page/v0.1" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	root := t.TempDir()
	r := Registry{BaseURL: srv.URL, Client: srv.Client()}

	wantPath := filepath.Join(root, ".dev", "contracts", "umono", "page", "v0.1.toml")
	path, err := EnsureContract(context.Background(), root, "umono/page@v0.1", r)
	if err != nil {
		t.Fatalf("EnsureContract first call: %v", err)
	}
	if path != wantPath {
		t.Fatalf("path = %q, want %q", path, wantPath)
	}
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if string(got) != "first" {
		t.Fatalf("cache contents = %q, want %q", got, "first")
	}
	if count != 1 {
		t.Fatalf("count after first call = %d, want 1", count)
	}

	body = "second"
	path, err = EnsureContract(context.Background(), root, "umono/page@v0.1", r)
	if err != nil {
		t.Fatalf("EnsureContract second call: %v", err)
	}
	if path != wantPath {
		t.Fatalf("path = %q, want %q", path, wantPath)
	}
	got, err = os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if string(got) != "first" {
		t.Fatalf("cache contents after second call = %q, want %q", got, "first")
	}
	if count != 1 {
		t.Fatalf("count after second call = %d, want 1", count)
	}
}

func TestEnsureContractNotPublished(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	root := t.TempDir()
	r := Registry{BaseURL: srv.URL, Client: srv.Client()}
	cachePath := filepath.Join(root, ".dev", "contracts", "umono", "page", "v0.1.toml")

	if _, err := EnsureContract(context.Background(), root, "umono/page@v0.1", r); !errors.Is(err, ErrContractNotPublished) {
		t.Fatalf("EnsureContract err = %v, want ErrContractNotPublished", err)
	}
	if _, err := os.Stat(cachePath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(%s) err = %v, want fs.ErrNotExist", cachePath, err)
	}
}

func TestEnsureContractUserAgent(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("x"))
	}))
	t.Cleanup(srv.Close)

	root := t.TempDir()
	r := Registry{BaseURL: srv.URL, Client: srv.Client()}
	if _, err := EnsureContract(context.Background(), root, "umono/page@v0.1", r); err != nil {
		t.Fatalf("EnsureContract: %v", err)
	}
	if ua != "Umono" {
		t.Fatalf("User-Agent = %q, want %q", ua, "Umono")
	}
}

func TestEnsureContractUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("x"))
	}))
	url := srv.URL
	client := srv.Client()
	srv.Close()

	root := t.TempDir()
	r := Registry{BaseURL: url, Client: client}
	if _, err := EnsureContract(context.Background(), root, "umono/page@v0.1", r); !errors.Is(err, ErrRegistryUnreachable) {
		t.Fatalf("EnsureContract err = %v, want ErrRegistryUnreachable", err)
	}
}
