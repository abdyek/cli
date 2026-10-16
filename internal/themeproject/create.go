package themeproject

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/umono-cms/themecore"
)

// ErrExists is returned when a file or a directory already has the name of the
// project. Nothing is overwritten.
var ErrExists = errors.New("already exists")

// cssHeader starts assets/theme.css. Transform drops comments, so it does not
// go into the package.
const cssHeader = "/* The starting point of the theme: Umono's default look,\n * themecore.FallbackSource(). Edit it freely.\n */\n\n"

// Create validates m and creates the theme project dir with the boilerplate
// files: theme.toml, assets/theme.css and .gitignore. Once dir is made, an
// error removes it, so a failed run leaves nothing behind.
func Create(dir string, m Manifest) (err error) {
	if err := m.Validate(); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%s: %w", dir, ErrExists)
		}
		return err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dir)
		}
	}()
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		return err
	}
	files := []struct {
		path string
		data []byte
	}{
		{"theme.toml", Render(m)},
		{filepath.Join("assets", "theme.css"), append([]byte(cssHeader), themecore.FallbackSource()...)},
		{".gitignore", []byte(".dev/\ndist/\n")},
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.path), f.data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
