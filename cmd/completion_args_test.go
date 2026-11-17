package cmd

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// complete runs the hidden __complete command with the given command line and
// returns the suggested completions together with the completion directive.
//
// Cobra prints one suggestion per line and a last line holding the directive as
// ":<n>", where n is the cobra.ShellCompDirective as an integer.
func complete(t *testing.T, args ...string) ([]string, cobra.ShellCompDirective) {
	t.Helper()

	output, err := runRootCmd(t, append([]string{"__complete"}, args...)...)
	if err != nil {
		t.Fatalf("rootCmd.Execute() error = %v", err)
	}

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	directiveIndex := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], ":") {
			directiveIndex = i
			break
		}
	}
	if directiveIndex < 0 {
		t.Fatalf("completion output %q has no directive line", output)
	}

	value, err := strconv.Atoi(strings.TrimPrefix(lines[directiveIndex], ":"))
	if err != nil {
		t.Fatalf("strconv.Atoi(%q) error = %v", strings.TrimPrefix(lines[directiveIndex], ":"), err)
	}

	suggestions := make([]string, 0, directiveIndex)
	for _, line := range lines[:directiveIndex] {
		if line == "" {
			continue
		}
		suggestions = append(suggestions, line)
	}

	return suggestions, cobra.ShellCompDirective(value)
}

// completionCommands returns the command and all of its subcommands,
// recursively.
func completionCommands(c *cobra.Command) []*cobra.Command {
	commands := []*cobra.Command{c}
	for _, sub := range c.Commands() {
		commands = append(commands, completionCommands(sub)...)
	}
	return commands
}

func TestCompletionArgsSuggestNoFiles(t *testing.T) {
	for _, c := range completionCommands(rootCmd) {
		if c.Hidden || c.CommandPath() == "umono theme install" {
			continue
		}

		t.Run(c.CommandPath(), func(t *testing.T) {
			args := append(strings.Fields(c.CommandPath())[1:], "")

			_, directive := complete(t, args...)
			if directive&cobra.ShellCompDirectiveNoFileComp == 0 {
				t.Fatalf("directive = %v, want ShellCompDirectiveNoFileComp bit set", directive)
			}
		})
	}
}

func TestCompletionFlagValuesSuggestNoFiles(t *testing.T) {
	for _, c := range completionCommands(rootCmd) {
		if c.Hidden {
			continue
		}

		c.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
			if flag.Value.Type() == "bool" {
				return
			}

			t.Run(c.CommandPath()+" --"+flag.Name, func(t *testing.T) {
				args := append(strings.Fields(c.CommandPath())[1:], "--"+flag.Name, "")

				_, directive := complete(t, args...)
				if directive&cobra.ShellCompDirectiveNoFileComp == 0 {
					t.Fatalf("directive = %v, want ShellCompDirectiveNoFileComp bit set", directive)
				}
			})
		})
	}
}

func TestThemeInstallCompletesThemeFiles(t *testing.T) {
	t.Run("completes theme files", func(t *testing.T) {
		suggestions, directive := complete(t, "theme", "install", "")

		want := []string{"umono-theme"}
		if !reflect.DeepEqual(suggestions, want) {
			t.Fatalf("suggestions = %v, want %v", suggestions, want)
		}
		if directive != cobra.ShellCompDirectiveFilterFileExt {
			t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveFilterFileExt)
		}
	})

	t.Run("does not complete more files", func(t *testing.T) {
		_, directive := complete(t, "theme", "install", "a.umono-theme", "")
		if directive&cobra.ShellCompDirectiveNoFileComp == 0 {
			t.Fatalf("directive = %v, want ShellCompDirectiveNoFileComp bit set", directive)
		}
	})
}
