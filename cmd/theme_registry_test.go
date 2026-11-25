package cmd

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/themeproject"
)

// TestThemeCommandsRejectInvalidRegistry checks that the commands that can
// reach the registry fail at start on an invalid UMONO_REGISTRY, even when the
// contract is cached.
func TestThemeCommandsRejectInvalidRegistry(t *testing.T) {
	t.Setenv("UMONO_REGISTRY", "ftp://registry.example.lan")
	t.Chdir(t.TempDir())

	cases := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{name: "build", cmd: themeBuildCmd},
		{name: "dev", cmd: themeDevCmd},
		{name: "create", cmd: themeCreateCmd, args: []string{"my-theme"}},
	}

	for _, tc := range cases {
		tc.cmd.SetContext(context.Background())
		err := tc.cmd.RunE(tc.cmd, tc.args)
		if !errors.Is(err, themeproject.ErrRegistryURL) {
			t.Fatalf("%s: err = %v, want themeproject.ErrRegistryURL", tc.name, err)
		}
	}

	for _, path := range []string{"my-theme", ".dev"} {
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("os.Stat(%s) err = %v, want fs.ErrNotExist", path, err)
		}
	}
}
