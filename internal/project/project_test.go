package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/umono-cms/cli/internal/umonoproject"
)

func TestUpgradeRequiresAUmonoProject(t *testing.T) {
	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "umono-linux-amd64")
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	err := Upgrade(dir)
	if !errors.Is(err, umonoproject.ErrNotUmonoProject) {
		t.Fatalf("err = %v, want ErrNotUmonoProject", err)
	}
}

func TestUpgradeLooksForTheBinaryAfterDetection(t *testing.T) {
	dir := t.TempDir()

	umonoJSONPath := filepath.Join(dir, "umono.json")
	if err := os.WriteFile(umonoJSONPath, []byte(`{"min_cli_version": "0.2.2"}`), 0o644); err != nil {
		t.Fatalf("write umono.json: %v", err)
	}

	err := Upgrade(dir)
	if err == nil {
		t.Fatalf("Upgrade returned nil error, want non-nil")
	}
	if errors.Is(err, umonoproject.ErrNotUmonoProject) {
		t.Fatalf("err = %v should not satisfy ErrNotUmonoProject", err)
	}
	if !strings.Contains(err.Error(), "binary") {
		t.Fatalf("err = %v, want it to contain %q", err, "binary")
	}
}
