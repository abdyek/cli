package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/umono-cms/cli/internal/umonoproject"
	"github.com/umono-cms/themecore"
)

// themeBreak is an installed theme that a Umono release would not activate.
type themeBreak struct {
	// Theme is "{org}/{id} {version}", or the path under themes/ when its
	// built.toml cannot be read.
	Theme  string
	Reason string
}

// checkThemes returns the installed themes of projectDir, the directories
// themes/{org}/{id}/{version}/, that the release in releaseDir would not
// activate: a Theme API version outside the range of the release's
// umono.json, or a built.toml that cannot be read. themes/dev holds the
// theme of umono theme dev and is skipped. Pending installs are not checked,
// because applying one runs the checks of activation.
func checkThemes(projectDir, releaseDir string) ([]themeBreak, error) {
	m, err := umonoproject.Detect(releaseDir)
	if err != nil {
		return nil, err
	}

	noThemes := false
	min, current, err := m.ThemeAPI()
	if err != nil {
		if errors.Is(err, umonoproject.ErrNoThemeSupport) {
			noThemes = true
		} else {
			return nil, err
		}
	}

	themesDir := filepath.Join(projectDir, "themes")
	orgs, err := os.ReadDir(themesDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var breaks []themeBreak
	for _, org := range orgs {
		if !org.IsDir() || org.Name() == "dev" {
			continue
		}
		ids, err := os.ReadDir(filepath.Join(themesDir, org.Name()))
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if !id.IsDir() {
				continue
			}
			versions, err := os.ReadDir(filepath.Join(themesDir, org.Name(), id.Name()))
			if err != nil {
				return nil, err
			}
			for _, version := range versions {
				if !version.IsDir() {
					continue
				}
				rel := org.Name() + "/" + id.Name() + "/" + version.Name()
				b, err := os.ReadFile(filepath.Join(themesDir, rel, "built.toml"))
				if err != nil {
					breaks = append(breaks, themeBreak{Theme: rel, Reason: "built.toml cannot be read: " + err.Error()})
					continue
				}
				built, err := themecore.ParseBuilt(b)
				if err != nil {
					breaks = append(breaks, themeBreak{Theme: rel, Reason: "built.toml cannot be read: " + err.Error()})
					continue
				}
				if noThemes {
					breaks = append(breaks, themeBreak{Theme: built.ID + " " + built.Version, Reason: "the new Umono does not support themes"})
					continue
				}
				if built.ThemeAPIVersion < min || built.ThemeAPIVersion > current {
					breaks = append(breaks, themeBreak{
						Theme:  built.ID + " " + built.Version,
						Reason: fmt.Sprintf("Theme API %d, supported %d\u2013%d", built.ThemeAPIVersion, min, current),
					})
				}
			}
		}
	}

	return breaks, nil
}
