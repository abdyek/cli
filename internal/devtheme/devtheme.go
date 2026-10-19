// Package devtheme keeps the dev directory of .dev/server,
// themes/dev/dev/0.0.0/, in sync with a theme project during umono theme dev.
package devtheme

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/umono-cms/cli/internal/themeproject"
	"github.com/umono-cms/themecore"
)

// Path is the dev directory under .dev/server.
const Path = "themes/dev/dev/0.0.0"

// State is the outcome of Sync, what the dev proxy shows in its overlay.
type State struct {
	// Contract is the contract reference of theme.toml.
	Contract string
	// Err is the error of the checks of build, nil when they pass.
	Err error
	// Warnings are those of Lint when the checks pass.
	Warnings []themecore.Warning
}

// Sync brings the dev directory under serverDir up to date with the theme
// project projectDir, on start and on every change of the project. The
// contract is the cached file of theme.toml's reference, fetched from r
// when missing. The links are made (makeLinks) and built.toml is rewritten with
// the id dev/dev, the version 0.0.0 and the theme-api-version of the
// current detection. When the checks of build fail, the version is the
// CLI's ThemeAPICurrent, so Umono still loads the theme; the failure is
// State.Err. An error is returned when the dev directory cannot be brought
// up to date: theme.toml or the contract cannot be had, or a write fails.
func Sync(ctx context.Context, projectDir, serverDir string, r themeproject.Registry, now time.Time) (State, error) {
	b, err := os.ReadFile(filepath.Join(projectDir, "theme.toml"))
	if err != nil {
		return State{}, err
	}
	m, err := themecore.ParseManifest(b)
	if err != nil {
		return State{}, err
	}
	state := State{Contract: m.Contract}
	contractPath, err := themeproject.EnsureContract(ctx, projectDir, m.Contract, r)
	if err != nil {
		return state, err
	}
	contract, err := os.ReadFile(contractPath)
	if err != nil {
		return state, err
	}
	dir := filepath.Join(serverDir, filepath.FromSlash(Path))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return state, err
	}
	if err := makeLinks(projectDir, dir, contractPath); err != nil {
		return state, err
	}

	version := themecore.ThemeAPICurrent()
	if _, report, err := themecore.CheckSource(projectDir, contract); err != nil {
		state.Err = err
	} else {
		version = report.Version
		state.Warnings = report.Warnings
	}
	m.ID, m.Version = "dev/dev", "0.0.0"
	if err := writeFileAtomic(filepath.Join(dir, "built.toml"), themeproject.RenderBuilt(m, version, now)); err != nil {
		return state, err
	}
	return state, nil
}

// makeLinks makes the links of the dev directory dir: assets/ and
// attributes.toml into projectDir, and contract.toml to contractPath, a
// file of the project's contract cache. The targets are absolute. A link is
// replaced atomically, and the link of a missing attributes.toml is
// removed.
func makeLinks(projectDir, dir, contractPath string) error {
	root, err := filepath.Abs(projectDir)
	if err != nil {
		return err
	}
	contract, err := filepath.Abs(contractPath)
	if err != nil {
		return err
	}
	if err := symlink(filepath.Join(root, "assets"), filepath.Join(dir, "assets")); err != nil {
		return err
	}
	if err := symlink(contract, filepath.Join(dir, "contract.toml")); err != nil {
		return err
	}
	attributes := filepath.Join(root, "attributes.toml")
	if _, err := os.Stat(attributes); errors.Is(err, fs.ErrNotExist) {
		if err := os.Remove(filepath.Join(dir, "attributes.toml")); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}
	return symlink(attributes, filepath.Join(dir, "attributes.toml"))
}

// symlink makes name a symlink to target, replacing name atomically.
func symlink(target, name string) error {
	tmp := filepath.Join(filepath.Dir(name), ".tmp-"+filepath.Base(name))
	os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, name); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// writeFileAtomic writes b to path through a temporary file in the same
// directory, so Umono never reads a half-written built.toml.
func writeFileAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
