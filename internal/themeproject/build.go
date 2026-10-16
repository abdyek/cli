package themeproject

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
	"github.com/umono-cms/themecore"
)

// ErrNotThemeProject is returned when the directory has no theme.toml.
var ErrNotThemeProject = errors.New("not a theme project: theme.toml is missing")

// BuildResult is the outcome of Build.
type BuildResult struct {
	// Path is the built file, dist/{org}_{id}_{version}.umono-theme.
	Path   string
	Report themecore.BuildReport
	// PreservedComment reports a comment starting with /*! in
	// assets/theme.css. Comments do not go into the package; a note meant
	// to stay belongs in LICENSE.
	PreservedComment bool
}

// Build checks the theme project dir and writes its built file into dist/.
// The contract is the cached file of the reference in theme.toml, fetched
// from r when missing. dist/ is touched only after every check has passed.
func Build(ctx context.Context, dir string, r Registry, now time.Time) (BuildResult, error) {
	manifest, err := os.ReadFile(filepath.Join(dir, "theme.toml"))
	if errors.Is(err, fs.ErrNotExist) {
		return BuildResult{}, fmt.Errorf("%s: %w", dir, ErrNotThemeProject)
	}
	if err != nil {
		return BuildResult{}, err
	}
	m, err := themecore.ParseManifest(manifest)
	if err != nil {
		return BuildResult{}, err
	}
	contractPath, err := EnsureContract(ctx, dir, m.Contract, r)
	if err != nil {
		return BuildResult{}, err
	}
	contract, err := os.ReadFile(contractPath)
	if err != nil {
		return BuildResult{}, err
	}
	p, report, err := themecore.CheckSource(dir, contract)
	if err != nil {
		return BuildResult{}, err
	}
	source, err := os.ReadFile(filepath.Join(dir, "assets", "theme.css"))
	if err != nil {
		return BuildResult{}, err
	}
	built := RenderBuilt(m, report.Version, now)
	if _, err := themecore.ParseBuilt(built); err != nil {
		return BuildResult{}, err
	}
	p.Files["built.toml"] = built
	sums, err := themecore.Checksums(p)
	if err != nil {
		return BuildResult{}, err
	}
	p.Files["checksums.json"] = sums
	archive, err := zipPackage(p, now)
	if err != nil {
		return BuildResult{}, err
	}
	dist := filepath.Join(dir, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return BuildResult{}, err
	}
	path := filepath.Join(dist, strings.ReplaceAll(m.ID, "/", "_")+"_"+m.Version+".umono-theme")
	if err := writeFileAtomic(path, archive); err != nil {
		return BuildResult{}, err
	}
	return BuildResult{Path: path, Report: report, PreservedComment: hasPreservedComment(source)}, nil
}

// zipPackage returns p as a zip of its files in path order. Only files are
// written, so an empty directory of the development directory does not go
// into the package.
func zipPackage(p themecore.Package, modified time.Time) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, path := range slices.Sorted(maps.Keys(p.Files)) {
		f, err := w.CreateHeader(&zip.FileHeader{Name: path, Method: zip.Deflate, Modified: modified})
		if err != nil {
			return nil, err
		}
		if _, err := f.Write(p.Files[path]); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// hasPreservedComment reports whether the CSS source has a comment starting
// with /*!, the convention for a note that minifiers keep.
func hasPreservedComment(source []byte) bool {
	l := css.NewLexer(parse.NewInputBytes(source))
	for {
		tt, data := l.Next()
		switch tt {
		case css.ErrorToken:
			return false
		case css.CommentToken:
			if bytes.HasPrefix(data, []byte("/*!")) {
				return true
			}
		}
	}
}
