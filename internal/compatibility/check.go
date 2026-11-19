package compatibility

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/umono-cms/cli/internal/download"
	"github.com/umono-cms/cli/internal/version"
	"github.com/umono-cms/themecore"
)

// releaseVersion matches a release version, MAJOR.MINOR.PATCH with an
// optional leading "v" and nothing else.
var releaseVersion = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+$`)

// warnings is where a development build writes that it skips the minimum
// CLI version check; tests replace it.
var warnings io.Writer = os.Stderr

const CLIUpgradeURL = "https://docs.umono.dev/getting-started/installation/#update-the-cli"

// ErrContractOutdated is returned when the theme's contract cannot be read
// because the CLI is out of date. It replaces an error that wraps
// themecore.ErrOutdated, which means the contract uses Compono syntax or a
// component that the CLI's themecore does not know.
var ErrContractOutdated = errors.New("the CLI is out of date and cannot read the theme's contract")

// OutdatedContract translates an error that wraps themecore.ErrOutdated into
// ErrContractOutdated with upgrade instructions. Any other error is returned
// unchanged.
func OutdatedContract(err error) error {
	if errors.Is(err, themecore.ErrOutdated) {
		return fmt.Errorf("%w; upgrade the CLI: %s", ErrContractOutdated, CLIUpgradeURL)
	}
	return err
}

type CheckResult struct {
	Compatible    bool
	CLIVersion    string
	MinCLIVersion string
	UmonoVersion  string
}

func Check(client *download.Client) (*CheckResult, error) {
	manifest, err := client.GetManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to check compatibility: %w", err)
	}

	releaseInfo, err := client.GetLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("failed to get release info: %w", err)
	}

	compatible := compatibleWith(manifest.MinCLIVersion, releaseInfo.Version)

	return &CheckResult{
		Compatible:    compatible,
		CLIVersion:    version.Version,
		MinCLIVersion: manifest.MinCLIVersion,
		UmonoVersion:  releaseInfo.Version,
	}, nil
}

func CheckForVersion(client *download.Client, umonoVersion string) (*CheckResult, error) {
	manifest, err := client.GetManifestForVersion(umonoVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to check compatibility: %w", err)
	}

	compatible := compatibleWith(manifest.MinCLIVersion, umonoVersion)

	return &CheckResult{
		Compatible:    compatible,
		CLIVersion:    version.Version,
		MinCLIVersion: manifest.MinCLIVersion,
		UmonoVersion:  umonoVersion,
	}, nil
}

func FormatIncompatibleError(result *CheckResult) string {
	return fmt.Sprintf(`❌ CLI version incompatible

Your CLI version:     %s
Required CLI version: %s (minimum)
Umono version:        %s

Upgrade the CLI to install this version of Umono:
  → %s
`, result.CLIVersion, result.MinCLIVersion, result.UmonoVersion, CLIUpgradeURL)
}

// isRelease reports whether v is a release version, MAJOR.MINOR.PATCH with an
// optional leading "v" and nothing else. Any other version, like 0.0.0-dev, a
// pseudo-version or a version with a "+dirty" suffix, is a development build.
func isRelease(v string) bool {
	return releaseVersion.MatchString(v)
}

// compatibleWith reports whether the CLI is compatible with the minimum CLI
// version minVersion of the Umono release umonoVersion. A development build
// skips the check and is always compatible.
func compatibleWith(minVersion, umonoVersion string) bool {
	if !isRelease(version.Version) {
		fmt.Fprintf(warnings, "warning: development build %s skips the minimum CLI version check (Umono %s requires %s)\n", version.Display(), umonoVersion, minVersion)
		return true
	}

	return isVersionCompatible(version.Version, minVersion)
}

func isVersionCompatible(cliVersion, minVersion string) bool {
	cliParts := parseVersion(cliVersion)
	minParts := parseVersion(minVersion)

	for i := 0; i < 3; i++ {
		if cliParts[i] > minParts[i] {
			return true
		}
		if cliParts[i] < minParts[i] {
			return false
		}
	}

	return true
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")

	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		numStr := strings.Split(parts[i], "-")[0]
		num, _ := strconv.Atoi(numStr)
		result[i] = num
	}

	return result
}
