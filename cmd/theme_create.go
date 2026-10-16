package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/themeproject"
	"golang.org/x/term"
)

var themeCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a theme project",
	Long:  "Create a theme project in a new directory. Flags answer the questions; the missing ones are asked. With --no-input, or when standard input is not a terminal, nothing is asked.",
	Args:  cobra.ExactArgs(1),
	RunE:  runThemeCreateCmd,
}

func init() {
	for _, f := range createFields {
		themeCreateCmd.Flags().String(f.flag, "", f.prompt)
	}
	themeCreateCmd.Flags().Bool("no-input", false, "Ask nothing; a missing required flag is an error")
}

func runThemeCreateCmd(cmd *cobra.Command, args []string) error {
	given := map[string]string{}
	for _, f := range createFields {
		if cmd.Flags().Changed(f.flag) {
			given[f.flag], _ = cmd.Flags().GetString(f.flag)
		}
	}
	noInput, _ := cmd.Flags().GetBool("no-input")
	interactive := !noInput && term.IsTerminal(int(os.Stdin.Fd()))
	return runThemeCreate(cmd.Context(), args[0], given, interactive, os.Stdin, cmd.OutOrStdout(), themeproject.DefaultRegistry)
}

// createField is a question of umono theme create. The flag of the same name
// answers it.
type createField struct {
	flag     string
	prompt   string
	required bool
	def      string // used when the flag is not given or the answer is empty
	set      func(*themeproject.Manifest, string)
}

var createFields = []createField{
	{flag: "id", prompt: "Theme ID (org/id)", required: true, set: func(m *themeproject.Manifest, v string) { m.ID = v }},
	{flag: "name", prompt: "Theme name", required: true, set: func(m *themeproject.Manifest, v string) { m.Name = v }},
	{flag: "contract", prompt: "Contract (org/id@vMAJOR.MINOR)", required: true, set: func(m *themeproject.Manifest, v string) { m.Contract = v }},
	{flag: "version", prompt: "Version", def: "0.1.0", set: func(m *themeproject.Manifest, v string) { m.Version = v }},
	{flag: "description", prompt: "Description", set: func(m *themeproject.Manifest, v string) { m.Description = v }},
	{flag: "license", prompt: "License (SPDX expression)", set: func(m *themeproject.Manifest, v string) { m.License = v }},
	{flag: "homepage", prompt: "Homepage (https://)", set: func(m *themeproject.Manifest, v string) { m.Homepage = v }},
	{flag: "repository", prompt: "Repository (https://)", set: func(m *themeproject.Manifest, v string) { m.Repository = v }},
	{flag: "author-name", prompt: "Author name", set: func(m *themeproject.Manifest, v string) { m.AuthorName = v }},
	{flag: "author-url", prompt: "Author URL (https://)", set: func(m *themeproject.Manifest, v string) { m.AuthorURL = v }},
}

// checkField validates value as the field alone: the other fields hold valid
// placeholders, so the rules are those of theme.toml.
func checkField(f createField, value string) error {
	m := themeproject.Manifest{ID: "acme/theme", Name: "Theme", Contract: "umono/page@v0.1", Version: "0.1.0", AuthorName: "Author"}
	f.set(&m, value)
	return m.Validate()
}

// errNoAnswer is returned when the input ends before a question is answered.
var errNoAnswer = errors.New("input ended before an answer")

// ask asks prompt until check accepts the answer. An empty answer is def.
func ask(in *bufio.Reader, out io.Writer, prompt, def string, check func(string) error) (string, error) {
	for {
		if def != "" {
			fmt.Fprintf(out, "%s [%s]: ", prompt, def)
		} else {
			fmt.Fprintf(out, "%s: ", prompt)
		}
		line, err := in.ReadString('\n')
		if err != nil && (!errors.Is(err, io.EOF) || line == "") {
			return "", fmt.Errorf("%s: %w", prompt, errNoAnswer)
		}
		answer := strings.TrimSpace(line)
		if answer == "" {
			answer = def
		}
		if err := check(answer); err != nil {
			fmt.Fprintf(out, "%v\nPlease try again.\n", err)
			continue
		}
		return answer, nil
	}
}

// askField asks f. The contract is asked in two parts, org/id and the
// version, that make the same org/id@vMAJOR.MINOR value as the flag.
func askField(in *bufio.Reader, out io.Writer, f createField) (string, error) {
	check := func(v string) error { return checkField(f, v) }
	if f.flag != "contract" {
		return ask(in, out, f.prompt, f.def, check)
	}
	id, err := ask(in, out, "Contract (org/id)", "", func(v string) error { return check(v + "@v0.1") })
	if err != nil {
		return "", err
	}
	version, err := ask(in, out, "Contract version (vMAJOR.MINOR)", "", func(v string) error { return check("umono/page@" + v) })
	if err != nil {
		return "", err
	}
	return id + "@" + version, nil
}

// runThemeCreate creates the theme project name in the working directory.
// given holds the flags given on the command line. In interactive mode the
// missing fields are asked; otherwise a missing required flag is an error.
// Everything is validated before anything is written.
func runThemeCreate(ctx context.Context, name string, given map[string]string, interactive bool, in io.Reader, out io.Writer, r themeproject.Registry) error {
	if _, err := os.Lstat(name); err == nil {
		return fmt.Errorf("%s: %w", name, themeproject.ErrExists)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, f := range createFields {
		if v, ok := given[f.flag]; ok {
			if err := checkField(f, v); err != nil {
				return fmt.Errorf("--%s: %w", f.flag, err)
			}
		}
	}
	values := maps.Clone(given)
	if values == nil {
		values = map[string]string{}
	}
	if interactive {
		reader := bufio.NewReader(in)
		for _, f := range createFields {
			if _, ok := values[f.flag]; ok {
				continue
			}
			if f.flag == "author-url" && values["author-name"] == "" {
				continue
			}
			v, err := askField(reader, out, f)
			if err != nil {
				return err
			}
			values[f.flag] = v
		}
	} else {
		var missing []string
		for _, f := range createFields {
			if _, ok := values[f.flag]; !ok && f.required {
				missing = append(missing, "--"+f.flag)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing required flags: %s", strings.Join(missing, ", "))
		}
	}
	var m themeproject.Manifest
	for _, f := range createFields {
		v, ok := values[f.flag]
		if !ok || v == "" {
			v = f.def
		}
		f.set(&m, v)
	}
	if err := themeproject.Create(name, m); err != nil {
		return err
	}
	fmt.Fprintf(out, "Created the theme project %s.\n", name)
	if _, err := themeproject.EnsureContract(ctx, name, m.Contract, r); err != nil {
		fmt.Fprintf(out, "Warning: the contract %s could not be cached: %v\numono theme dev and umono theme build fetch it when it is missing.\n", m.Contract, err)
	}
	if m.License != "" {
		fmt.Fprintf(out, "theme.toml has license %s, but the project has no LICENSE file. Add the license text to the project root as LICENSE.\n", m.License)
	}
	fmt.Fprintf(out, "Next: cd %s\n", shellQuote(name))
	if interactive {
		fmt.Fprintf(out, "\nTo create the same project without questions:\n  %s\n", equivalentCommand(name, values))
	}
	return nil
}

// equivalentCommand is the umono theme create command that makes the same
// project without questions. Empty values and defaults are left out.
func equivalentCommand(name string, values map[string]string) string {
	args := []string{"umono", "theme", "create", shellQuote(name)}
	for _, f := range createFields {
		v := values[f.flag]
		if v == "" || v == f.def {
			continue
		}
		args = append(args, "--"+f.flag, shellQuote(v))
	}
	return strings.Join(args, " ")
}

// squote is the single quote of a POSIX shell. A single quote inside a quoted
// value is written as squote, backslash, squote, squote.
const squote = string(rune(39))

// shellQuote returns s as a POSIX shell word: as it is when it has only safe
// characters, single-quoted otherwise.
func shellQuote(s string) string {
	if s != "" && !strings.ContainsFunc(s, func(r rune) bool { return !shellSafe(r) }) {
		return s
	}
	return squote + strings.ReplaceAll(s, squote, squote+string(rune(92))+squote+squote) + squote
}

func shellSafe(r rune) bool {
	return r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("@%+=:,./_-", r))
}
