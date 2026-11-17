package cmd

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func runRootCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs(args)

	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	err := rootCmd.Execute()
	return out.String(), err
}

func TestCompletionScripts(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			output, err := runRootCmd(t, "completion", shell)
			if err != nil {
				t.Fatalf("rootCmd.Execute() error = %v", err)
			}

			if output == "" {
				t.Fatalf("rootCmd.Execute() output is empty")
			}

			for _, want := range []string{"umono", "__complete"} {
				if !strings.Contains(output, want) {
					t.Fatalf("output %q does not contain %q", output, want)
				}
			}

			for _, unwanted := range []string{"theme", "install"} {
				if strings.Contains(output, unwanted) {
					t.Fatalf("output %q should not contain %q", output, unwanted)
				}
			}
		})
	}
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	if _, err := runRootCmd(t, "completion", "powershell"); err == nil {
		t.Fatalf("rootCmd.Execute() error = nil, want error for unsupported shell")
	}
}

func TestCompletionRequiresOneShell(t *testing.T) {
	t.Run("missing shell", func(t *testing.T) {
		if _, err := runRootCmd(t, "completion"); err == nil {
			t.Fatalf("rootCmd.Execute() error = nil, want error when shell is missing")
		}
	})

	t.Run("too many shells", func(t *testing.T) {
		if _, err := runRootCmd(t, "completion", "bash", "zsh"); err == nil {
			t.Fatalf("rootCmd.Execute() error = nil, want error when more than one shell is given")
		}
	})
}
