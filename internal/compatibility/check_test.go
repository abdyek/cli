package compatibility

import (
	"bytes"
	"strings"
	"testing"

	"github.com/umono-cms/cli/internal/download"
	"github.com/umono-cms/cli/internal/download/downloadtest"
	"github.com/umono-cms/cli/internal/version"
)

func TestIsVersionCompatible(t *testing.T) {
	tests := []struct {
		name       string
		cliVersion string
		minVersion string
		want       bool
	}{
		{"equal versions", "0.1.0", "0.1.0", true},
		{"equal with v prefix", "v0.1.0", "0.1.0", true},

		{"cli major newer", "1.0.0", "0.1.0", true},
		{"cli minor newer", "0.2.0", "0.1.0", true},
		{"cli patch newer", "0.1.1", "0.1.0", true},
		{"cli much newer", "2.5.3", "0.1.0", true},

		{"cli major older", "0.1.0", "1.0.0", false},
		{"cli minor older", "0.1.0", "0.2.0", false},
		{"cli patch older", "0.1.0", "0.1.1", false},
		{"cli much older", "0.1.0", "2.5.3", false},

		{"min version 0.0.0", "0.1.0", "0.0.0", true},
		{"both 0.0.0", "0.0.0", "0.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isVersionCompatible(tt.cliVersion, tt.minVersion)
			if got != tt.want {
				t.Errorf("isVersionCompatible(%q, %q) = %v, want %v",
					tt.cliVersion, tt.minVersion, got, tt.want)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
	}{
		{"0.1.0", [3]int{0, 1, 0}},
		{"v0.1.0", [3]int{0, 1, 0}},
		{"1.2.3", [3]int{1, 2, 3}},
		{"v1.2.3", [3]int{1, 2, 3}},
		{"0.1", [3]int{0, 1, 0}},
		{"1", [3]int{1, 0, 0}},
		{"1.0.0-beta", [3]int{1, 0, 0}},
		{"2.1.3-rc1", [3]int{2, 1, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseVersion(tt.input)
			if got != tt.want {
				t.Errorf("parseVersion(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsRelease(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"development build", "0.0.0-dev", false},
		{"development build with revision", "v0.0.0-dev+abc123def456", false},
		{"pseudo version", "v0.3.1-0.20261119130550-bbd70e59f02b", false},
		{"pseudo version with dirty", "v0.3.1-0.20261119130550-bbd70e59f02b+dirty", false},
		{"not a version", "dev", false},

		{"release without v prefix", "0.4.0", true},
		{"release with v prefix", "v0.4.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRelease(tt.version)
			if got != tt.want {
				t.Fatalf("isRelease(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestCheckSkipsTheMinimumForADevelopmentBuild(t *testing.T) {
	files := map[string][]byte{
		"umono.json": []byte(`{"min_cli_version": "99.0.0", "theme_api_current": 1, "theme_api_min": 1}`),
	}

	srv := downloadtest.New(t, "v0.9.0", files)

	client, err := download.NewClientWithURL(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithURL(%q): %v", srv.URL, err)
	}

	oldVersion := version.Version
	oldWarnings := warnings
	t.Cleanup(func() {
		version.Version = oldVersion
		warnings = oldWarnings
	})

	tests := []struct {
		name        string
		cliVersion  string
		want        bool
		wantWarning string
	}{
		{
			name:        "development build skips the minimum",
			cliVersion:  "0.0.0-dev",
			want:        true,
			wantWarning: "warning: development build v0.0.0-dev skips the minimum CLI version check (Umono v0.9.0 requires 99.0.0)\n",
		},
		{
			name:        "release build does not skip the minimum",
			cliVersion:  "0.4.0",
			want:        false,
			wantWarning: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version.Version = tt.cliVersion

			buf := &bytes.Buffer{}
			warnings = buf

			result, err := Check(client)
			if err != nil {
				t.Fatalf("version %q: Check: %v", tt.cliVersion, err)
			}
			if result.Compatible != tt.want {
				t.Fatalf("version %q: Check: Compatible = %v, want %v", tt.cliVersion, result.Compatible, tt.want)
			}
			if got := buf.String(); got != tt.wantWarning {
				t.Fatalf("version %q: Check: warnings = %q, want %q", tt.cliVersion, got, tt.wantWarning)
			}

			buf = &bytes.Buffer{}
			warnings = buf

			result, err = CheckForVersion(client, "v0.9.0")
			if err != nil {
				t.Fatalf("version %q: CheckForVersion: %v", tt.cliVersion, err)
			}
			if result.Compatible != tt.want {
				t.Fatalf("version %q: CheckForVersion: Compatible = %v, want %v", tt.cliVersion, result.Compatible, tt.want)
			}
			if got := buf.String(); got != tt.wantWarning {
				t.Fatalf("version %q: CheckForVersion: warnings = %q, want %q", tt.cliVersion, got, tt.wantWarning)
			}
		})
	}
}

func TestFormatIncompatibleErrorGivesTheDocsAddress(t *testing.T) {
	result := &CheckResult{CLIVersion: "v0.3.0", MinCLIVersion: "v0.4.0", UmonoVersion: "v0.10.0"}

	want := "Upgrade the CLI to install this version of Umono:\n  → https://docs.umono.dev/getting-started/installation/#update-the-cli\n"

	got := FormatIncompatibleError(result)
	if !strings.Contains(got, want) {
		t.Errorf("FormatIncompatibleError(result) = %q, want it to contain %q", got, want)
	}
}
