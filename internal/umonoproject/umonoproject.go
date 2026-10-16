// Package umonoproject detects a Umono project by its umono.json.
package umonoproject

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	ErrNotUmonoProject = errors.New("this directory is not a Umono project")
	ErrNoThemeSupport  = errors.New("this Umono version does not support themes, v0.9 or later is required")
)

// Manifest is umono.json at the root of a Umono project. Every release
// archive has it, and upgrade keeps it current. The theme fields are nil in
// a project before v0.9.
type Manifest struct {
	MinCLIVersion   string `json:"min_cli_version"`
	ThemeAPICurrent *int   `json:"theme_api_current"`
	ThemeAPIMin     *int   `json:"theme_api_min"`
}

// Detect reads umono.json at the root dir. A missing file, or one that is
// not a JSON object, means dir is not a Umono project. The binary's name is
// not used, and parent directories are not searched.
func Detect(dir string) (Manifest, error) {
	b, err := os.ReadFile(filepath.Join(dir, "umono.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: umono.json: %v", ErrNotUmonoProject, err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(b, &object); err != nil || object == nil {
		return Manifest{}, fmt.Errorf("%w: umono.json is not a JSON object", ErrNotUmonoProject)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("%w: umono.json: %v", ErrNotUmonoProject, err)
	}
	return m, nil
}

// ThemeAPI returns the Theme API range of the project's Umono.
func (m Manifest) ThemeAPI() (min, current int, err error) {
	if m.ThemeAPICurrent == nil && m.ThemeAPIMin == nil {
		return 0, 0, ErrNoThemeSupport
	}
	if m.ThemeAPICurrent == nil || m.ThemeAPIMin == nil {
		return 0, 0, errors.New("umono.json: theme_api_current and theme_api_min must be set together")
	}
	min, current = *m.ThemeAPIMin, *m.ThemeAPICurrent
	if min < 1 || min > current {
		return 0, 0, fmt.Errorf("umono.json: invalid Theme API range %d to %d", min, current)
	}
	return min, current, nil
}
