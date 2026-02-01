package compatibility

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/umono-cms/cli/internal/download"
	"github.com/umono-cms/cli/internal/version"
)

const CLIUpgradeURL = "https://umono.io/cli"

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

	compatible := isVersionCompatible(version.Version, manifest.MinCLIVersion)

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

	compatible := isVersionCompatible(version.Version, manifest.MinCLIVersion)

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

Please upgrade your CLI to install this version of Umono:
  → %s
`, result.CLIVersion, result.MinCLIVersion, result.UmonoVersion, CLIUpgradeURL)
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
