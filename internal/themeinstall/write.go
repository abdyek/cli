package themeinstall

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/umono-cms/themecore"
)

// writePackage writes p into dir through a staging directory next to it, so
// that dir appears complete or not at all. Files get mode 0444 and
// directories 0555. With replace, an existing dir (a pending install of the
// same version) is replaced.
func writePackage(dir string, p themecore.Package, replace bool) error {
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".install-*")
	if err != nil {
		return err
	}
	defer removeAll(stage)
	for path, data := range p.Files {
		target := filepath.Join(stage, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o444); err != nil {
			return err
		}
		if err := os.Chmod(target, 0o444); err != nil {
			return err
		}
	}
	var dirs []string
	if err := filepath.WalkDir(stage, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != stage {
			dirs = append(dirs, path)
		}
		return nil
	}); err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chmod(dirs[i], 0o555); err != nil {
			return err
		}
	}
	if replace {
		if err := removeAll(dir); err != nil {
			return err
		}
	}
	// The staging directory stays writable until it is renamed, then it
	// becomes read-only too.
	if err := os.Rename(stage, dir); err != nil {
		return err
	}
	return os.Chmod(dir, 0o555)
}

// removeAll removes dir, whose directories may be read-only.
func removeAll(dir string) error {
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			os.Chmod(path, 0o755)
		}
		return nil
	})
	return os.RemoveAll(dir)
}
