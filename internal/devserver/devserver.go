// Package devserver manages .dev/server, the Umono project that umono theme
// dev runs a theme project with.
package devserver

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/umono-cms/cli/internal/download"
	"github.com/umono-cms/cli/internal/project"
	"github.com/umono-cms/cli/internal/umonoproject"
	"github.com/umono-cms/themecore"
)

// Username and Password are the account of .dev/server.
const (
	Username = "admin"
	Password = "admin"
)

// ErrCLIOutdated is returned when no theme the CLI builds works with the
// runtime of .dev/server.
var ErrCLIOutdated = errors.New("the CLI is outdated")

// Dir is .dev/server of the theme project projectDir.
func Dir(projectDir string) string {
	return filepath.Join(projectDir, ".dev", "server")
}

// Ensure sets .dev/server up when it has no umono.json: an empty Umono
// project of the latest release of client, with the account admin/admin.
// It is set up in a temporary directory next to it and renamed into place,
// so a failed setup leaves nothing. A .dev/server without a valid
// umono.json is not touched: it may hold the theme author's umono.db. It
// reports whether it set the server up.
func Ensure(projectDir string, client *download.Client) (bool, error) {
	dir := Dir(projectDir)
	if _, err := umonoproject.Detect(dir); err == nil {
		return false, nil
	}
	if _, err := os.Stat(dir); err == nil {
		return false, fmt.Errorf("%s is not a Umono project; delete it to set it up again", dir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return false, err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), ".server-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tmp)
	if err := project.CreateWith(client, project.Project{Username: Username, Password: Password, Path: tmp, Port: "8999"}); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, dir); err != nil {
		return false, err
	}
	return true, nil
}

// CheckThemeAPI compares the Theme API range in the umono.json of
// .dev/server with the CLI's themecore. The same range returns nothing. A
// runtime whose theme_api_min is above the CLI's ThemeAPICurrent runs no
// theme the CLI builds: ErrCLIOutdated. Any other difference is a warning.
func CheckThemeAPI(projectDir string) (string, error) {
	m, err := umonoproject.Detect(Dir(projectDir))
	if err != nil {
		return "", err
	}
	min, current, err := m.ThemeAPI()
	if err != nil {
		return "", err
	}
	cliMin, cliCurrent := themecore.ThemeAPIMin(), themecore.ThemeAPICurrent()
	switch {
	case min == cliMin && current == cliCurrent:
		return "", nil
	case cliCurrent < min:
		return "", fmt.Errorf("%w: .dev/server supports Theme API %d to %d, and the CLI builds up to %d", ErrCLIOutdated, min, current, cliCurrent)
	}
	return fmt.Sprintf("The Theme API of the runtime (%d to %d) differs from the CLI's (%d to %d). Update the runtime with: umono theme dev --update-server", min, current, cliMin, cliCurrent), nil
}

// Update runs the flow of umono upgrade on .dev/server: its binary and
// umono.json are replaced with those of the latest release of client, and
// umono.db is kept. Installed themes do not stop it: the only theme of
// .dev/server is dev/dev, which upgrade skips.
func Update(projectDir string, client *download.Client) error {
	return project.UpgradeWith(client, Dir(projectDir), false)
}
