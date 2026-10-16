package cmd

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/themeproject"
	"github.com/umono-cms/themecore"
)

var themeBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the theme into dist/",
	Long:  "Check the theme project in the working directory and build it into dist/{org}_{id}_{version}.umono-theme.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runThemeBuild(cmd.Context(), ".", cmd.OutOrStdout(), themeproject.DefaultRegistry, time.Now())
	},
}

// runThemeBuild builds the theme project dir and prints theme-api-version
// with its reasons, the warnings and the built file.
func runThemeBuild(ctx context.Context, dir string, out io.Writer, r themeproject.Registry, now time.Time) error {
	res, err := themeproject.Build(ctx, dir, r, now)
	if err != nil {
		return err
	}
	fmt.Fprint(out, formatEvidence(res.Report.Version, res.Report.Evidence, themecore.ThemeAPIMin()))
	fmt.Fprint(out, formatWarnings(res.Report.Warnings))
	if res.PreservedComment {
		fmt.Fprintln(out, "warning: assets/theme.css: a comment starting with /*! does not go into the package; move the note to LICENSE")
	}
	fmt.Fprintf(out, "Built %s\n", res.Path)
	return nil
}

// formatEvidence prints theme-api-version with its reasons: a line for each
// detected feature and one for the theme_api_min floor.
func formatEvidence(version int, evidence []themecore.Signal, min int) string {
	type row struct{ where, why string }
	rows := make([]row, 0, len(evidence)+1)
	for _, s := range evidence {
		rows = append(rows, row{s.Where, fmt.Sprintf("%s (API %d)", s.Feature, s.IntroducedIn)})
	}
	rows = append(rows, row{"theme_api_min", strconv.Itoa(min)})
	width := 0
	for _, r := range rows {
		width = max(width, len(r.where))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "theme-api-version: %d\n", version)
	for _, r := range rows {
		fmt.Fprintf(&b, "  ← %-*s  %s\n", width, r.where, r.why)
	}
	return b.String()
}

// formatWarnings prints the warnings of Lint, one per line.
func formatWarnings(warnings []themecore.Warning) string {
	var b strings.Builder
	for _, w := range warnings {
		fmt.Fprintf(&b, "warning: %s: %s\n", w.Where, w.Message)
	}
	return b.String()
}
