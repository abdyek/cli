package themeinstall

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/umono-cms/themecore"

	"github.com/umono-cms/cli/internal/themeproject"
	"github.com/umono-cms/cli/internal/umonoproject"
)

const v1JSON = `{"min_cli_version": "0.3.0", "theme_api_current": 1, "theme_api_min": 1}`

func makeWritable(dir string) {
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			_ = os.Chmod(path, 0o755)
		}
		return nil
	})
}

func umonoProject(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "umono.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile umono.json: %v", err)
	}
	t.Cleanup(func() { makeWritable(dir) })
	return dir
}

func builtPackage(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "themeproject", "testdata", "page-v0.1.toml"))
	if err != nil {
		t.Fatalf("ReadFile page-v0.1.toml: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/registry/v1/contracts/umono/page/v0.1" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	dir := filepath.Join(t.TempDir(), "theme")
	if err := themeproject.Create(dir, themeproject.Manifest{
		ID:       "acme/minimal",
		Name:     "Minimal",
		Contract: "umono/page@v0.1",
		Version:  "0.1.0",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	res, err := themeproject.Build(context.Background(), dir, themeproject.Registry{BaseURL: srv.URL, Client: srv.Client()}, time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return res.Path
}

func zipFiles(t *testing.T, path string) map[string][]byte {
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

func writeZip(t *testing.T, files map[string][]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.umono-theme")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create %s: %v", path, err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		h := &zip.FileHeader{Name: n, Method: zip.Deflate}
		h.SetMode(0o644)
		w2, err := w.CreateHeader(h)
		if err != nil {
			t.Fatalf("CreateHeader %s: %v", n, err)
		}
		if _, err := w2.Write(files[n]); err != nil {
			t.Fatalf("Write %s: %v", n, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip.Close: %v", err)
	}
	return path
}

func withChecksums(t *testing.T, files map[string][]byte) map[string][]byte {
	t.Helper()
	delete(files, "checksums.json")
	b, err := themecore.Checksums(themecore.Package{Files: files})
	if err != nil {
		t.Fatalf("themecore.Checksums: %v", err)
	}
	files["checksums.json"] = b
	return files
}

func treeFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir %s: %v", dir, err)
	}
	return files
}

func noThemes(t *testing.T, proj string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(proj, "themes")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(themes) err = %v, want fs.ErrNotExist", err)
	}
	if _, err := os.Stat(filepath.Join(proj, "themes-pending")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(themes-pending) err = %v, want fs.ErrNotExist", err)
	}
}

func installed(proj string) string {
	return filepath.Join(proj, "themes", "acme", "minimal", "0.1.0")
}

func TestInstall(t *testing.T) {
	proj := umonoProject(t, v1JSON)
	file := builtPackage(t)
	res, err := Install(proj, file, Options{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if res.Dir != installed(proj) {
		t.Fatalf("res.Dir = %q, want %q", res.Dir, installed(proj))
	}
	if res.Pending {
		t.Fatalf("res.Pending = true, want false")
	}
	if res.Incompatible {
		t.Fatalf("res.Incompatible = true, want false")
	}
	if res.Built.ID != "acme/minimal" {
		t.Fatalf("res.Built.ID = %q, want %q", res.Built.ID, "acme/minimal")
	}
	got := treeFiles(t, res.Dir)
	want := zipFiles(t, file)
	if len(got) != len(want) {
		t.Fatalf("installed file count = %d, want %d", len(got), len(want))
	}
	for n, b := range want {
		gb, ok := got[n]
		if !ok {
			t.Fatalf("installed missing %q", n)
		}
		if !bytes.Equal(gb, b) {
			t.Fatalf("installed %q content mismatch", n)
		}
	}
	err = filepath.WalkDir(res.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		st, err := os.Stat(path)
		if err != nil {
			return err
		}
		if st.Mode().Perm() != 0o444 {
			t.Fatalf("file %s perm = %o, want 0o444", path, st.Mode().Perm())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir %s: %v", res.Dir, err)
	}
	for _, dir := range []string{res.Dir, filepath.Join(res.Dir, "assets")} {
		st, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("Stat %s: %v", dir, err)
		}
		if st.Mode().Perm() != 0o555 {
			t.Fatalf("dir %s perm = %o, want 0o555", dir, st.Mode().Perm())
		}
	}
	p, err := themecore.ReadDir(res.Dir)
	if err != nil {
		t.Fatalf("themecore.ReadDir(%s): %v", res.Dir, err)
	}
	if err := themecore.VerifyChecksums(p); err != nil {
		t.Fatalf("VerifyChecksums: %v", err)
	}
	if _, err := themecore.CheckPackage(p); err != nil {
		t.Fatalf("CheckPackage: %v", err)
	}
}

func TestInstallNotAUmonoProject(t *testing.T) {
	file := builtPackage(t)
	_, err := Install(t.TempDir(), file, Options{})
	if !errors.Is(err, umonoproject.ErrNotUmonoProject) {
		t.Fatalf("Install err = %v, want ErrNotUmonoProject", err)
	}
}

func TestInstallBeforeV09(t *testing.T) {
	proj := umonoProject(t, `{"min_cli_version": "0.2.2"}`)
	file := builtPackage(t)
	_, err := Install(proj, file, Options{})
	if !errors.Is(err, umonoproject.ErrNoThemeSupport) {
		t.Fatalf("Install err = %v, want ErrNoThemeSupport", err)
	}
	noThemes(t, proj)
}

func TestInstallNotAZip(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x.umono-theme")
	if err := os.WriteFile(file, []byte("not a zip"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	proj := umonoProject(t, v1JSON)
	if _, err := Install(proj, file, Options{}); err == nil {
		t.Fatalf("Install = nil, want error")
	}
	noThemes(t, proj)
}

func TestInstallTooLarge(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "x.umono-theme"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := f.Truncate(64<<20 + 1); err != nil {
		t.Fatalf("Truncate: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	proj := umonoProject(t, v1JSON)
	if _, err := Install(proj, f.Name(), Options{}); err == nil {
		t.Fatalf("Install = nil, want error")
	}
	noThemes(t, proj)
}

func TestInstallChecksumMismatch(t *testing.T) {
	files := zipFiles(t, builtPackage(t))
	files["assets/theme.css"] = append(files["assets/theme.css"], '\n')
	proj := umonoProject(t, v1JSON)
	if _, err := Install(proj, writeZip(t, files), Options{}); err == nil {
		t.Fatalf("Install = nil, want error")
	}
	noThemes(t, proj)
}

func TestInstallCLIOutdated(t *testing.T) {
	files := zipFiles(t, builtPackage(t))
	v, err := themecore.ParseBuilt(files["built.toml"])
	if err != nil {
		t.Fatalf("ParseBuilt: %v", err)
	}
	from := "theme-api-version = " + strconv.Itoa(v.ThemeAPIVersion)
	to := "theme-api-version = " + strconv.Itoa(themecore.ThemeAPICurrent()+1)
	if from == to {
		t.Fatalf("theme-api-version rewrite produces the same string: %s", from)
	}
	rewritten := bytes.Replace(files["built.toml"], []byte(from), []byte(to), 1)
	if bytes.Equal(rewritten, files["built.toml"]) {
		t.Fatalf("theme-api-version replacement did not change built.toml:\n%s", files["built.toml"])
	}
	files["built.toml"] = rewritten
	file := writeZip(t, withChecksums(t, files))
	for _, opts := range []Options{{}, {AllowIncompatibleAPI: true}} {
		proj := umonoProject(t, v1JSON)
		_, err := Install(proj, file, opts)
		if !errors.Is(err, ErrCLIOutdated) {
			t.Fatalf("Install opts=%+v err = %v, want ErrCLIOutdated", opts, err)
		}
		noThemes(t, proj)
	}
}

func TestInstallDevDev(t *testing.T) {
	files := zipFiles(t, builtPackage(t))
	from := "id = " + strconv.Quote("acme/minimal")
	to := "id = " + strconv.Quote("dev/dev")
	if from == to {
		t.Fatalf("id rewrite produces the same string")
	}
	rewritten := bytes.Replace(files["built.toml"], []byte(from), []byte(to), 1)
	if bytes.Equal(rewritten, files["built.toml"]) {
		t.Fatalf("id replacement did not change built.toml:\n%s", files["built.toml"])
	}
	files["built.toml"] = rewritten
	proj := umonoProject(t, v1JSON)
	if _, err := Install(proj, writeZip(t, withChecksums(t, files)), Options{}); err == nil {
		t.Fatalf("Install = nil, want error")
	}
	noThemes(t, proj)
}

func TestInstallAlreadyInstalled(t *testing.T) {
	proj := umonoProject(t, v1JSON)
	file := builtPackage(t)
	if _, err := Install(proj, file, Options{}); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	before := treeFiles(t, installed(proj))
	_, err := Install(proj, file, Options{})
	if !errors.Is(err, ErrAlreadyInstalled) {
		t.Fatalf("second Install err = %v, want ErrAlreadyInstalled", err)
	}
	if !reflect.DeepEqual(treeFiles(t, installed(proj)), before) {
		t.Fatalf("installed files changed on second install")
	}
	if _, err := os.Stat(filepath.Join(proj, "themes-pending")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(themes-pending) err = %v, want fs.ErrNotExist", err)
	}
}

func TestInstallForce(t *testing.T) {
	proj := umonoProject(t, v1JSON)
	file := builtPackage(t)
	if _, err := Install(proj, file, Options{}); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	before := treeFiles(t, installed(proj))
	res, err := Install(proj, file, Options{Force: true})
	if err != nil {
		t.Fatalf("Install Force: %v", err)
	}
	if !res.Pending {
		t.Fatalf("res.Pending = false, want true")
	}
	wantDir := filepath.Join(proj, "themes-pending", "acme", "minimal", "0.1.0")
	if res.Dir != wantDir {
		t.Fatalf("res.Dir = %q, want %q", res.Dir, wantDir)
	}
	got := treeFiles(t, res.Dir)
	want := zipFiles(t, file)
	if len(got) != len(want) {
		t.Fatalf("pending file count = %d, want %d", len(got), len(want))
	}
	for n, b := range want {
		gb, ok := got[n]
		if !ok {
			t.Fatalf("pending missing %q", n)
		}
		if !bytes.Equal(gb, b) {
			t.Fatalf("pending %q content mismatch", n)
		}
	}
	err = filepath.WalkDir(res.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		st, err := os.Stat(path)
		if err != nil {
			return err
		}
		if st.Mode().Perm() != 0o444 {
			t.Fatalf("pending file %s perm = %o, want 0o444", path, st.Mode().Perm())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir pending: %v", err)
	}
	err = filepath.WalkDir(res.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		st, err := os.Stat(path)
		if err != nil {
			return err
		}
		if st.Mode().Perm() != 0o555 {
			t.Fatalf("pending dir %s perm = %o, want 0o555", path, st.Mode().Perm())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir pending dirs: %v", err)
	}
	if !reflect.DeepEqual(treeFiles(t, installed(proj)), before) {
		t.Fatalf("installed files changed while installing to pending")
	}
	res2, err := Install(proj, file, Options{Force: true})
	if err != nil {
		t.Fatalf("second Install Force: %v", err)
	}
	got2 := treeFiles(t, res2.Dir)
	if !reflect.DeepEqual(got2, want) {
		t.Fatalf("pending files changed after second force install")
	}
	entries, err := os.ReadDir(filepath.Join(proj, "themes-pending", "acme", "minimal"))
	if err != nil {
		t.Fatalf("ReadDir themes-pending/acme/minimal: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "0.1.0" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Fatalf("themes-pending/acme/minimal entries = %v, want [0.1.0]", names)
	}
}

func TestInstallForceWithoutAnInstall(t *testing.T) {
	proj := umonoProject(t, v1JSON)
	file := builtPackage(t)
	res, err := Install(proj, file, Options{Force: true})
	if err != nil {
		t.Fatalf("Install Force: %v", err)
	}
	if res.Pending {
		t.Fatalf("res.Pending = true, want false")
	}
	if res.Dir != installed(proj) {
		t.Fatalf("res.Dir = %q, want %q", res.Dir, installed(proj))
	}
}

func TestInstallIncompatibleAPI(t *testing.T) {
	proj := umonoProject(t, `{"min_cli_version": "0.3.0", "theme_api_current": 2, "theme_api_min": 2}`)
	file := builtPackage(t)
	if _, err := Install(proj, file, Options{}); !errors.Is(err, ErrIncompatibleAPI) {
		t.Fatalf("Install err = %v, want ErrIncompatibleAPI", err)
	}
	noThemes(t, proj)
	res, err := Install(proj, file, Options{AllowIncompatibleAPI: true})
	if err != nil {
		t.Fatalf("Install AllowIncompatibleAPI: %v", err)
	}
	if !res.Incompatible {
		t.Fatalf("res.Incompatible = false, want true")
	}
	if res.Dir != installed(proj) {
		t.Fatalf("res.Dir = %q, want %q", res.Dir, installed(proj))
	}
}
