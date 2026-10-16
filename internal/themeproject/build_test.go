package themeproject

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/umono-cms/themecore"
)

var now = time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)

const wantName = "acme_minimal_0.1.0.umono-theme"

func pageRegistry(t *testing.T) Registry {
	t.Helper()
	body := readTestdata(t, "page-v0.1.toml")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/registry/v1/contracts/umono/page/v0.1" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return Registry{BaseURL: srv.URL, Client: srv.Client()}
}

func newProject(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "theme")
	if err := Create(dir, minimal); err != nil {
		t.Fatalf("Create(%s): %v", dir, err)
	}
	return dir
}

func readZip(t *testing.T, path string) map[string][]byte {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("zip.OpenReader(%s): %v", path, err)
	}
	defer r.Close()
	files := make(map[string][]byte)
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("zip entry %s Open: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("zip entry %s ReadAll: %v", f.Name, err)
		}
		files[f.Name] = b
	}
	return files
}

func TestRenderBuilt(t *testing.T) {
	m, err := themecore.ParseManifest(readTestdata(t, "full.toml"))
	if err != nil {
		t.Fatalf("ParseManifest(full.toml): %v", err)
	}
	b := RenderBuilt(m, 1, now)
	if string(b) != string(readTestdata(t, "built-full.toml")) {
		t.Fatalf("RenderBuilt mismatch:\n--- got ---\n%s\n--- want ---\n%s", b, readTestdata(t, "built-full.toml"))
	}
	if _, err := themecore.ParseBuilt(b); err != nil {
		t.Fatalf("ParseBuilt: %v", err)
	}
}

func TestBuild(t *testing.T) {
	dir := newProject(t)
	res, err := Build(context.Background(), dir, pageRegistry(t), now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if res.Path != filepath.Join(dir, "dist", wantName) {
		t.Fatalf("res.Path = %q, want %q", res.Path, filepath.Join(dir, "dist", wantName))
	}
	files := readZip(t, res.Path)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	wantNames := []string{"assets/theme.css", "built.toml", "checksums.json", "contract.toml"}
	if len(names) != len(wantNames) {
		t.Fatalf("zip names = %v, want %v", names, wantNames)
	}
	for i, n := range names {
		if n != wantNames[i] {
			t.Fatalf("zip names = %v, want %v", names, wantNames)
		}
	}
	if string(files["contract.toml"]) != string(readTestdata(t, "page-v0.1.toml")) {
		t.Fatalf("contract.toml mismatch:\n--- got ---\n%s\n--- want ---\n%s", files["contract.toml"], readTestdata(t, "page-v0.1.toml"))
	}
	m, _ := themecore.ParseManifest(readTestdata(t, "minimal.toml"))
	wantBuilt := RenderBuilt(m, res.Report.Version, now)
	if string(files["built.toml"]) != string(wantBuilt) {
		t.Fatalf("built.toml mismatch:\n--- got ---\n%s\n--- want ---\n%s", files["built.toml"], wantBuilt)
	}
	if res.Report.Version < themecore.ThemeAPIMin() {
		t.Fatalf("res.Report.Version = %d, want >= %d", res.Report.Version, themecore.ThemeAPIMin())
	}
	if res.PreservedComment {
		t.Fatalf("res.PreservedComment = true, want false")
	}
	f, err := os.Open(res.Path)
	if err != nil {
		t.Fatalf("os.Open(%s): %v", res.Path, err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		t.Fatalf("f.Stat: %v", err)
	}
	p, err := themecore.ReadZip(f, st.Size())
	if err != nil {
		t.Fatalf("ReadZip: %v", err)
	}
	if err := themecore.VerifyChecksums(p); err != nil {
		t.Fatalf("VerifyChecksums: %v", err)
	}
	if _, err := themecore.CheckPackage(p); err != nil {
		t.Fatalf("CheckPackage: %v", err)
	}
}

func TestBuildSkipsEmptyDirectories(t *testing.T) {
	dir := newProject(t)
	if err := os.MkdirAll(filepath.Join(dir, "assets", "fonts", "empty"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	res, err := Build(context.Background(), dir, pageRegistry(t), now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	files := readZip(t, res.Path)
	for n := range files {
		if strings.HasPrefix(n, "assets/fonts") {
			t.Fatalf("zip contains %q (empty directory should be skipped)", n)
		}
	}
}

func TestBuildIncludesLicense(t *testing.T) {
	dir := newProject(t)
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte("MIT License\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	res, err := Build(context.Background(), dir, pageRegistry(t), now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	files := readZip(t, res.Path)
	if string(files["LICENSE"]) != "MIT License\n" {
		t.Fatalf("LICENSE in zip = %q, want %q", files["LICENSE"], "MIT License\n")
	}
}

func TestBuildPreservedComment(t *testing.T) {
	dir := newProject(t)
	css := filepath.Join(dir, "assets", "theme.css")
	old, err := os.ReadFile(css)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := append([]byte("/*! (c) Acme */\n"), old...)
	if err := os.WriteFile(css, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	res, err := Build(context.Background(), dir, pageRegistry(t), now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !res.PreservedComment {
		t.Fatalf("res.PreservedComment = false, want true")
	}
	files := readZip(t, res.Path)
	if strings.Contains(string(files["assets/theme.css"]), "(c) Acme") {
		t.Fatalf("packaged assets/theme.css contains preserved comment:\n%s", files["assets/theme.css"])
	}
}

func TestBuildReplacesThePreviousFile(t *testing.T) {
	dir := newProject(t)
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dist", wantName), []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	res, err := Build(context.Background(), dir, pageRegistry(t), now)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	files := readZip(t, res.Path)
	if _, ok := files["built.toml"]; !ok {
		t.Fatalf("zip %s does not contain built.toml", res.Path)
	}
}

func TestBuildFailureLeavesDistUntouched(t *testing.T) {
	dir := newProject(t)
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dist", wantName), []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "Bad.png"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := Build(context.Background(), dir, pageRegistry(t), now); err == nil {
		t.Fatalf("Build = nil, want error")
	}
	got, err := os.ReadFile(filepath.Join(dir, "dist", wantName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "old" {
		t.Fatalf("dist/%s = %q, want %q", wantName, got, "old")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "dist"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("dist entries = %d, want 1", len(entries))
	}
}

func TestBuildNotAThemeProject(t *testing.T) {
	_, err := Build(context.Background(), t.TempDir(), pageRegistry(t), now)
	if !errors.Is(err, ErrNotThemeProject) {
		t.Fatalf("Build err = %v, want ErrNotThemeProject", err)
	}
}

func TestBuildRejectsDevDev(t *testing.T) {
	dir := newProject(t)
	tomlPath := filepath.Join(dir, "theme.toml")
	orig, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	from := "id = " + strconv.Quote("acme/minimal")
	to := "id = " + strconv.Quote("dev/dev")
	rewritten := strings.Replace(string(orig), from, to, 1)
	if rewritten == string(orig) {
		t.Fatalf("rewrite did not match theme.toml:\n%s", orig)
	}
	if err := os.WriteFile(tomlPath, []byte(rewritten), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := Build(context.Background(), dir, pageRegistry(t), now); err == nil {
		t.Fatalf("Build = nil, want error")
	}
	if _, err := os.Stat(filepath.Join(dir, "dist")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(dist) err = %v, want fs.ErrNotExist", err)
	}
}
