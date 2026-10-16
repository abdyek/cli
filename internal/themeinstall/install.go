// Package themeinstall installs a built theme into a Umono project.
package themeinstall

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/umono-cms/cli/internal/umonoproject"
	"github.com/umono-cms/themecore"
)

var (
	ErrCLIOutdated      = errors.New("the CLI is out of date and cannot verify this theme; upgrade the CLI")
	ErrAlreadyInstalled = errors.New("the theme is already installed; use --force to reinstall it")
	ErrIncompatibleAPI  = errors.New("the theme's Theme API version is outside the range of this Umono; use --allow-incompatible-api to install it anyway")
)

// Options are the flags of umono theme install.
type Options struct {
	// Force reinstalls an installed theme. The installed directory is not
	// touched: the package goes to themes-pending/ and waits for the site
	// owner's approval in the admin panel.
	Force bool
	// AllowIncompatibleAPI installs a theme whose theme-api-version is
	// outside the range in umono.json. It does not cover ErrCLIOutdated.
	AllowIncompatibleAPI bool
}

// Result is the outcome of Install.
type Result struct {
	Dir          string
	Pending      bool
	Built        themecore.Built
	Incompatible bool
}

// Install checks the built file and installs it into the Umono project
// projectDir as themes/{org}/{id}/{version}/. It runs the themecore checks
// of activation in the same order; the checks that need the database or the
// network are the runtime's. Nothing is written before every check has
// passed.
func Install(projectDir, file string, opts Options) (Result, error) {
	manifest, err := umonoproject.Detect(projectDir)
	if err != nil {
		return Result{}, err
	}
	min, current, err := manifest.ThemeAPI()
	if err != nil {
		return Result{}, err
	}
	p, err := readPackage(file)
	if err != nil {
		return Result{}, err
	}
	b, ok := p.Files["built.toml"]
	if !ok {
		return Result{}, errors.New("built.toml: missing")
	}
	if err := themecore.VerifyChecksums(p); err != nil {
		return Result{}, err
	}
	built, err := themecore.ParseBuilt(b)
	if err != nil {
		return Result{}, err
	}
	if built.ThemeAPIVersion > themecore.ThemeAPICurrent() {
		return Result{}, fmt.Errorf("%w: theme-api-version is %d, the CLI supports up to %d", ErrCLIOutdated, built.ThemeAPIVersion, themecore.ThemeAPICurrent())
	}
	if _, err := themecore.CheckPackage(p); err != nil {
		return Result{}, err
	}
	rel := filepath.Join(filepath.FromSlash(built.ID), built.Version)
	dir := filepath.Join(projectDir, "themes", rel)
	pending := false
	if _, err := os.Lstat(dir); err == nil {
		if !opts.Force {
			return Result{}, fmt.Errorf("%s: %w", dir, ErrAlreadyInstalled)
		}
		dir = filepath.Join(projectDir, "themes-pending", rel)
		pending = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Result{}, err
	}
	incompatible := built.ThemeAPIVersion < min || built.ThemeAPIVersion > current
	if incompatible && !opts.AllowIncompatibleAPI {
		return Result{}, fmt.Errorf("%w: theme-api-version is %d, this Umono supports %d to %d", ErrIncompatibleAPI, built.ThemeAPIVersion, min, current)
	}
	if err := built.CheckInstallDir(dir); err != nil {
		return Result{}, err
	}
	if err := writePackage(dir, p, pending); err != nil {
		return Result{}, err
	}
	return Result{Dir: dir, Pending: pending, Built: built, Incompatible: incompatible}, nil
}

// readPackage reads the built file with themecore.ReadZip, which checks the
// file's size before anything else, then the zip entries, and counts the real
// bytes while decompressing.
func readPackage(file string) (themecore.Package, error) {
	f, err := os.Open(file)
	if err != nil {
		return themecore.Package{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return themecore.Package{}, err
	}
	if !st.Mode().IsRegular() {
		return themecore.Package{}, fmt.Errorf("%s: not a regular file", file)
	}
	return themecore.ReadZip(f, st.Size())
}
