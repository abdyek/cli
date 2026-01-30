package download

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/go-github/v68/github"
)

const (
	owner = "umono-cms"
	repo  = "umono"
)

type Client struct {
	gh *github.Client
}

func NewClient() *Client {
	return &Client{
		gh: github.NewClient(nil),
	}
}

type ReleaseInfo struct {
	Version   string
	AssetName string
	AssetURL  string
	AssetSize int64
}

func (c *Client) GetLatestRelease() (*ReleaseInfo, error) {
	ctx := context.Background()

	release, _, err := c.gh.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("could not get latest release: %w", err)
	}

	return c.findAssetForPlatform(release)
}

func (c *Client) findAssetForPlatform(release *github.RepositoryRelease) (*ReleaseInfo, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	platformName := platformToAssetName(osName, arch)

	for _, asset := range release.Assets {
		assetName := asset.GetName()

		if strings.Contains(assetName, platformName) && strings.HasSuffix(assetName, ".tar.gz") {
			return &ReleaseInfo{
				Version:   release.GetTagName(),
				AssetName: assetName,
				AssetURL:  asset.GetBrowserDownloadURL(),
				AssetSize: int64(asset.GetSize()),
			}, nil
		}
	}

	return nil, fmt.Errorf("no asset found for platform: %s (%s)", platformName, release.GetTagName())
}

func (c *Client) DownloadAndExtract(info *ReleaseInfo, destDir string) error {
	tmpFile, err := os.CreateTemp("", "umono-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	fmt.Printf("Downloading %s (%s)...\n", info.AssetName, info.Version)
	if err := downloadFile(info.AssetURL, tmpFile); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to reset file pointer: %w", err)
	}

	fmt.Printf("Extracting to %s...\n", destDir)
	if err := extractTarGz(tmpFile, destDir); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	fmt.Println("Download completed successfully!\n")
	return nil
}

func downloadFile(url string, dest io.Writer) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	_, err = io.Copy(dest, resp.Body)
	return err
}

func extractTarGz(src io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}

			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}

func platformToAssetName(os, arch string) string {
	osMap := map[string]string{
		"linux":  "Linux",
		"darwin": "Darwin",
	}

	archMap := map[string]string{
		"amd64": "x86_64",
		"arm64": "arm64",
	}

	osName, ok := osMap[os]
	if !ok {
		osName = strings.Title(os)
	}

	archName, ok := archMap[arch]
	if !ok {
		archName = arch
	}

	return fmt.Sprintf("%s_%s", osName, archName)
}
