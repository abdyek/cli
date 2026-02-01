package download

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/go-github/v68/github"
)

type Manifest struct {
	MinCLIVersion string `json:"min_cli_version"`
}

func (c *Client) GetManifest() (*Manifest, error) {
	ctx := context.Background()

	release, _, err := c.gh.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("could not get latest release: %w", err)
	}

	return c.fetchManifestFromRelease(release)
}

func (c *Client) GetManifestForVersion(version string) (*Manifest, error) {
	ctx := context.Background()

	release, _, err := c.gh.Repositories.GetReleaseByTag(ctx, owner, repo, version)
	if err != nil {
		return nil, fmt.Errorf("could not get release %s: %w", version, err)
	}

	return c.fetchManifestFromRelease(release)
}

func (c *Client) fetchManifestFromRelease(release *github.RepositoryRelease) (*Manifest, error) {
	var manifestURL string
	for _, asset := range release.Assets {
		if asset.GetName() == "umono.json" {
			manifestURL = asset.GetBrowserDownloadURL()
			break
		}
	}

	if manifestURL == "" {
		return &Manifest{MinCLIVersion: "0.0.0"}, nil
	}

	resp, err := http.Get(manifestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch manifest: HTTP %d", resp.StatusCode)
	}

	var manifest Manifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}
