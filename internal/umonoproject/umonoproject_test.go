package umonoproject

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func project(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "umono.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write umono.json: %v", err)
	}
	return dir
}

func TestDetect(t *testing.T) {
	dir := project(t, `{"min_cli_version": "0.3.0", "theme_api_current": 2, "theme_api_min": 1}`)
	m, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if m.MinCLIVersion != "0.3.0" {
		t.Fatalf("MinCLIVersion = %q, want %q", m.MinCLIVersion, "0.3.0")
	}
	min, current, err := m.ThemeAPI()
	if err != nil {
		t.Fatalf("ThemeAPI: %v", err)
	}
	if min != 1 {
		t.Fatalf("min = %d, want 1", min)
	}
	if current != 2 {
		t.Fatalf("current = %d, want 2", current)
	}
}

func TestDetectIgnoresTheBinary(t *testing.T) {
	dir := project(t, `{"min_cli_version": "0.3.0", "theme_api_current": 2, "theme_api_min": 1}`)
	if _, err := Detect(dir); err != nil {
		t.Fatalf("Detect: %v", err)
	}
}

func TestDetectMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := Detect(dir)
	if !errors.Is(err, ErrNotUmonoProject) {
		t.Fatalf("err = %v, want ErrNotUmonoProject", err)
	}
}

func TestDetectInvalid(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"unclosed_object", "{"},
		{"array", "[]"},
		{"null", "null"},
		{"string", `"text"`},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Detect(project(t, tc.content))
			if !errors.Is(err, ErrNotUmonoProject) {
				t.Fatalf("err = %v, want ErrNotUmonoProject", err)
			}
		})
	}
}

func TestDetectDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "umono.json"), 0o755); err != nil {
		t.Fatalf("mkdir umono.json: %v", err)
	}
	_, err := Detect(dir)
	if !errors.Is(err, ErrNotUmonoProject) {
		t.Fatalf("err = %v, want ErrNotUmonoProject", err)
	}
}

func TestThemeAPIBeforeV09(t *testing.T) {
	dir := project(t, `{"min_cli_version": "0.2.2"}`)
	m, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	_, _, err = m.ThemeAPI()
	if !errors.Is(err, ErrNoThemeSupport) {
		t.Fatalf("err = %v, want ErrNoThemeSupport", err)
	}
}

func TestThemeAPIInvalid(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"current_only", `{"theme_api_current": 1}`},
		{"min_only", `{"theme_api_min": 1}`},
		{"min_greater_than_current", `{"theme_api_current": 1, "theme_api_min": 2}`},
		{"both_zero", `{"theme_api_current": 0, "theme_api_min": 0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Detect(project(t, tc.content))
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			_, _, err = m.ThemeAPI()
			if err == nil {
				t.Fatalf("ThemeAPI returned nil error, want non-nil")
			}
			if errors.Is(err, ErrNoThemeSupport) {
				t.Fatalf("err = %v should not satisfy ErrNoThemeSupport", err)
			}
		})
	}
}
