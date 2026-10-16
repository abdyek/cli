package cmd

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/umono-cms/cli/internal/themeproject"
)

func newRegistry(t *testing.T, status int) themeproject.Registry {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/registry/v1/contracts/umono/page/v0.1" {
			http.NotFound(w, r)
			return
		}
		if status == http.StatusOK {
			_, _ = w.Write([]byte("contract"))
			return
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return themeproject.Registry{BaseURL: srv.URL, Client: srv.Client()}
}

var minimalTOML = string(themeproject.Render(themeproject.Manifest{
	ID:       "acme/minimal",
	Name:     "Minimal",
	Contract: "umono/page@v0.1",
	Version:  "0.1.0",
}))

var required = map[string]string{
	"id":       "acme/minimal",
	"name":     "Minimal",
	"contract": "umono/page@v0.1",
}

func TestThemeCreateWithFlags(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	err := runThemeCreate(context.Background(), "new-theme", required, false, strings.NewReader(""), &out, newRegistry(t, http.StatusOK))
	if err != nil {
		t.Fatalf("runThemeCreate() error = %v", err)
	}

	gotTOML, err := os.ReadFile("new-theme/theme.toml")
	if err != nil {
		t.Fatalf("read new-theme/theme.toml: %v", err)
	}
	if string(gotTOML) != minimalTOML {
		t.Fatalf("new-theme/theme.toml = %q, want %q", string(gotTOML), minimalTOML)
	}

	gotContract, err := os.ReadFile("new-theme/.dev/contracts/umono/page/v0.1.toml")
	if err != nil {
		t.Fatalf("read new-theme/.dev/contracts/umono/page/v0.1.toml: %v", err)
	}
	if string(gotContract) != "contract" {
		t.Fatalf("new-theme/.dev/contracts/umono/page/v0.1.toml = %q, want %q", string(gotContract), "contract")
	}

	output := out.String()
	if !strings.Contains(output, "cd new-theme") {
		t.Fatalf("output %q does not contain %q", output, "cd new-theme")
	}
	if strings.Contains(output, "umono theme create") {
		t.Fatalf("output %q should not contain %q", output, "umono theme create")
	}
	if strings.Contains(output, "LICENSE") {
		t.Fatalf("output %q should not contain %q", output, "LICENSE")
	}
}

func TestThemeCreateMissingFlags(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	given := map[string]string{"name": "Minimal"}

	err := runThemeCreate(context.Background(), "new-theme", given, false, strings.NewReader(""), &out, newRegistry(t, http.StatusOK))
	if err == nil {
		t.Fatalf("runThemeCreate() error = nil, want non-nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "--id") {
		t.Fatalf("error %q does not contain %q", msg, "--id")
	}
	if !strings.Contains(msg, "--contract") {
		t.Fatalf("error %q does not contain %q", msg, "--contract")
	}
	if strings.Contains(msg, "--name") {
		t.Fatalf("error %q should not contain %q", msg, "--name")
	}
	if _, err := os.Stat("new-theme"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(new-theme) err = %v, want fs.ErrNotExist", err)
	}
}

func TestThemeCreateInvalidFlagIsNotAsked(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	given := map[string]string{
		"id":       "dev/dev",
		"name":     required["name"],
		"contract": required["contract"],
	}

	err := runThemeCreate(context.Background(), "new-theme", given, true, strings.NewReader(strings.Repeat("acme/minimal\n", 20)), &out, newRegistry(t, http.StatusOK))
	if err == nil {
		t.Fatalf("runThemeCreate() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "--id") {
		t.Fatalf("error %q does not contain %q", err.Error(), "--id")
	}
	if _, err := os.Stat("new-theme"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(new-theme) err = %v, want fs.ErrNotExist", err)
	}
}

func TestThemeCreateAsksAndReasks(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	given := map[string]string{"name": "Minimal"}

	stdin := strings.NewReader(
		"dev/dev\n" +
			"acme/minimal\n" +
			"umono/page\n" +
			"0.1\n" +
			"v0.1\n" +
			strings.Repeat("\n", 8),
	)

	err := runThemeCreate(context.Background(), "new-theme", given, true, stdin, &out, newRegistry(t, http.StatusOK))
	if err != nil {
		t.Fatalf("runThemeCreate() error = %v", err)
	}

	gotTOML, err := os.ReadFile("new-theme/theme.toml")
	if err != nil {
		t.Fatalf("read new-theme/theme.toml: %v", err)
	}
	if string(gotTOML) != minimalTOML {
		t.Fatalf("new-theme/theme.toml = %q, want %q", string(gotTOML), minimalTOML)
	}

	const want = "umono theme create new-theme --id acme/minimal --name Minimal --contract umono/page@v0.1"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("output %q does not contain %q", out.String(), want)
	}
}

func TestThemeCreateQuotesTheEquivalentCommand(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	given := map[string]string{
		"id":       "acme/minimal",
		"contract": "umono/page@v0.1",
	}

	stdin := strings.NewReader("Minimal Blog\n" + strings.Repeat("\n", 8))

	err := runThemeCreate(context.Background(), "new-theme", given, true, stdin, &out, newRegistry(t, http.StatusOK))
	if err != nil {
		t.Fatalf("runThemeCreate() error = %v", err)
	}

	if !strings.Contains(out.String(), "--name 'Minimal Blog'") {
		t.Fatalf("output %q does not contain %q", out.String(), "--name 'Minimal Blog'")
	}
}

func TestThemeCreateContractFetchFailureIsAWarning(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	err := runThemeCreate(context.Background(), "new-theme", required, false, strings.NewReader(""), &out, newRegistry(t, http.StatusNotFound))
	if err != nil {
		t.Fatalf("runThemeCreate() error = %v", err)
	}

	if _, err := os.Stat("new-theme/theme.toml"); err != nil {
		t.Fatalf("os.Stat(new-theme/theme.toml) err = %v, want nil", err)
	}
	if _, err := os.Stat("new-theme/.dev"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(new-theme/.dev) err = %v, want fs.ErrNotExist", err)
	}
	if !strings.Contains(strings.ToLower(out.String()), "warning") {
		t.Fatalf("output %q does not contain %q", strings.ToLower(out.String()), "warning")
	}
}

func TestThemeCreateLicenseReminder(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	given := map[string]string{
		"id":       required["id"],
		"name":     required["name"],
		"contract": required["contract"],
		"license":  "MIT",
	}

	err := runThemeCreate(context.Background(), "new-theme", given, false, strings.NewReader(""), &out, newRegistry(t, http.StatusOK))
	if err != nil {
		t.Fatalf("runThemeCreate() error = %v", err)
	}

	if !strings.Contains(out.String(), "LICENSE") {
		t.Fatalf("output %q does not contain %q", out.String(), "LICENSE")
	}
}

func TestThemeCreateRefusesAnExistingPathBeforeAsking(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := os.Mkdir("new-theme", 0o755); err != nil {
		t.Fatalf("Mkdir(new-theme) err = %v", err)
	}

	var out bytes.Buffer
	given := map[string]string{"name": "Minimal"}

	err := runThemeCreate(context.Background(), "new-theme", given, true, strings.NewReader(strings.Repeat("x\n", 20)), &out, newRegistry(t, http.StatusOK))
	if !errors.Is(err, themeproject.ErrExists) {
		t.Fatalf("runThemeCreate() error = %v, want themeproject.ErrExists", err)
	}
	if out.Len() != 0 {
		t.Fatalf("output length = %d, want 0; output = %q", out.Len(), out.String())
	}
}

func TestThemeCreateStopsAtEndOfInput(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	given := map[string]string{"name": "Minimal"}

	err := runThemeCreate(context.Background(), "new-theme", given, true, strings.NewReader("acme/minimal\n"), &out, newRegistry(t, http.StatusOK))
	if err == nil {
		t.Fatalf("runThemeCreate() error = nil, want non-nil")
	}
	if _, err := os.Stat("new-theme"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(new-theme) err = %v, want fs.ErrNotExist", err)
	}
}

func TestThemeCreateFlags(t *testing.T) {
	names := []string{
		"id", "name", "contract", "version",
		"description", "license", "homepage", "repository",
		"author-name", "author-url", "no-input",
	}
	for _, name := range names {
		if themeCreateCmd.Flags().Lookup(name) == nil {
			t.Fatalf("themeCreateCmd.Flags().Lookup(%q) = nil, want non-nil", name)
		}
	}
}
