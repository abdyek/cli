package themeproject

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var (
	ErrInvalidContractRef   = errors.New("invalid contract reference")
	ErrContractNotPublished = errors.New("contract is not published on umono.dev")
	ErrRegistryUnreachable  = errors.New("umono.dev is unreachable")
)

// maxContractSize bounds a fetched contract.
const maxContractSize = 1 << 20

// contractRef is org/id@vMAJOR.MINOR. Groups 1, 3 and 5 are the org, the id
// and the version, the parts of the registry URL and of the cache path.
var contractRef = regexp.MustCompile(`^([a-z0-9]+(-[a-z0-9]+)*)/([a-z0-9]+(-[a-z0-9]+)*)@(v(0|[1-9][0-9]*)[.](0|[1-9][0-9]*))$`)

// Registry is where contracts are fetched from.
type Registry struct {
	BaseURL string
	Client  *http.Client
}

// DefaultRegistry is umono.dev, the only registry in v0.9.
var DefaultRegistry = Registry{BaseURL: "https://umono.dev", Client: &http.Client{Timeout: 30 * time.Second}}

// EnsureContract returns the cache path of ref in projectDir,
// .dev/contracts/{org}/{id}/v{major}.{minor}.toml. The file is fetched from r
// only when it is missing: contracts are immutable, so a cached file never
// changes.
func EnsureContract(ctx context.Context, projectDir, ref string, r Registry) (string, error) {
	m := contractRef.FindStringSubmatch(ref)
	if m == nil {
		return "", fmt.Errorf("%q: %w", ref, ErrInvalidContractRef)
	}
	org, id, version := m[1], m[3], m[5]
	path := filepath.Join(projectDir, ".dev", "contracts", org, id, version+".toml")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	b, err := r.fetch(ctx, org+"/"+id+"/"+version)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := writeFileAtomic(path, b); err != nil {
		return "", err
	}
	return path, nil
}

// fetch gets a contract with the least information: a GET with the
// version-less User-Agent Umono.
func (r Registry) fetch(ctx context.Context, contract string) ([]byte, error) {
	url := r.BaseURL + "/registry/v1/contracts/" + contract
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Umono")
	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRegistryUnreachable, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("%s: %w", contract, ErrContractNotPublished)
	default:
		return nil, fmt.Errorf("%w: %s", ErrRegistryUnreachable, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxContractSize+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRegistryUnreachable, err)
	}
	if len(b) > maxContractSize {
		return nil, fmt.Errorf("%s: contract is larger than %d bytes", contract, maxContractSize)
	}
	return b, nil
}

// writeFileAtomic writes b to path through a temporary file in the same
// directory. An interrupted write leaves no partial cache file, which would
// never be fetched again.
func writeFileAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
