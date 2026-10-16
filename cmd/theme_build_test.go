package cmd

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/umono-cms/cli/internal/themeproject"
	"github.com/umono-cms/themecore"
)

var buildNow = time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)

func pageContractRegistry(t *testing.T) themeproject.Registry {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "internal", "themeproject", "testdata", "page-v0.1.toml"))
	if err != nil {
		t.Fatalf("read page-v0.1.toml: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/registry/v1/contracts/umono/page/v0.1" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return themeproject.Registry{BaseURL: srv.URL, Client: srv.Client()}
}

func newThemeProject(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "theme")
	if err := themeproject.Create(dir, themeproject.Manifest{
		ID:       "acme/minimal",
		Name:     "Minimal",
		Contract: "umono/page@v0.1",
		Version:  "0.1.0",
	}); err != nil {
		t.Fatalf("Create(%s): %v", dir, err)
	}
	return dir
}

func TestFormatEvidence(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "evidence.txt"))
	if err != nil {
		t.Fatalf("read testdata/evidence.txt: %v", err)
	}
	got := formatEvidence(3, []themecore.Signal{
		{Feature: "assets.svg", IntroducedIn: 3, Where: "assets/icons/cart.svg"},
	}, 1)
	if got != string(want) {
		t.Fatalf("formatEvidence = %q, want %q", got, want)
	}
}

func TestFormatEvidenceMinOnly(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "evidence-min.txt"))
	if err != nil {
		t.Fatalf("read testdata/evidence-min.txt: %v", err)
	}
	got := formatEvidence(1, nil, 1)
	if got != string(want) {
		t.Fatalf("formatEvidence = %q, want %q", got, want)
	}
}

func TestFormatWarnings(t *testing.T) {
	got := formatWarnings([]themecore.Warning{
		{Message: "first", Where: "assets/theme.css"},
		{Message: "second", Where: "attributes.toml"},
	})
	want := "warning: assets/theme.css: first\nwarning: attributes.toml: second\n"
	if got != want {
		t.Fatalf("formatWarnings = %q, want %q", got, want)
	}
	if got := formatWarnings(nil); got != "" {
		t.Fatalf("formatWarnings(nil) = %q, want \"\"", got)
	}
}

func TestThemeBuild(t *testing.T) {
	reg := pageContractRegistry(t)
	t.Chdir(newThemeProject(t))

	var out bytes.Buffer
	if err := runThemeBuild(context.Background(), ".", &out, reg, buildNow); err != nil {
		t.Fatalf("runThemeBuild: %v", err)
	}

	output := out.String()
	for _, want := range []string{"theme-api-version: ", "theme_api_min", "dist/acme_minimal_0.1.0.umono-theme"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q does not contain %q", output, want)
		}
	}
	if strings.Contains(output, "/*!") {
		t.Fatalf("output %q should not contain %q", output, "/*!")
	}

	if _, err := os.Stat("dist/acme_minimal_0.1.0.umono-theme"); err != nil {
		t.Fatalf("os.Stat(dist/acme_minimal_0.1.0.umono-theme): %v", err)
	}
}

func TestThemeBuildPreservedComment(t *testing.T) {
	reg := pageContractRegistry(t)
	dir := newThemeProject(t)
	css := filepath.Join(dir, "assets", "theme.css")
	old, err := os.ReadFile(css)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", css, err)
	}
	content := append([]byte("/*! (c) Acme */\n"), old...)
	if err := os.WriteFile(css, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", css, err)
	}
	t.Chdir(dir)

	var out bytes.Buffer
	if err := runThemeBuild(context.Background(), ".", &out, reg, buildNow); err != nil {
		t.Fatalf("runThemeBuild: %v", err)
	}

	output := out.String()
	for _, want := range []string{"/*!", "LICENSE"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q does not contain %q", output, want)
		}
	}
}

func TestThemeBuildNotAThemeProject(t *testing.T) {
	reg := pageContractRegistry(t)
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	err := runThemeBuild(context.Background(), ".", &out, reg, buildNow)
	if !errors.Is(err, themeproject.ErrNotThemeProject) {
		t.Fatalf("runThemeBuild err = %v, want ErrNotThemeProject", err)
	}
}
