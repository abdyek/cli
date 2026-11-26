package themeproject

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/umono-cms/themecore"
)

// TestParseRegistryURL checks that ParseRegistryURL accepts only http and
// https root URLs without a query, a fragment or user info, and strips a
// trailing slash.
func TestParseRegistryURL(t *testing.T) {
	valid := []struct {
		in   string
		want string
	}{
		{"https://registry.example.lan", "https://registry.example.lan"},
		{"http://registry.example.lan:8080", "http://registry.example.lan:8080"},
		{"https://registry.example.lan/", "https://registry.example.lan"},
		{"https://example.lan/mirror/", "https://example.lan/mirror"},
	}
	for _, tc := range valid {
		got, err := ParseRegistryURL(tc.in)
		if err != nil {
			t.Fatalf("ParseRegistryURL(%q) err = %v, want nil", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseRegistryURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	invalid := []string{
		"",
		"registry.example.lan",
		"ftp://registry.example.lan",
		"https://",
		"https://registry.example.lan?x=1",
		"https://registry.example.lan/?",
		"https://registry.example.lan#top",
		"https://registry.example.lan/#",
		"https://user@registry.example.lan",
		"https://registry.example.lan/%zz",
	}
	for _, in := range invalid {
		got, err := ParseRegistryURL(in)
		if !errors.Is(err, ErrRegistryURL) {
			t.Fatalf("ParseRegistryURL(%q) err = %v, want ErrRegistryURL", in, err)
		}
		if got != "" {
			t.Fatalf("ParseRegistryURL(%q) = %q, want %q", in, got, "")
		}
	}
}

// TestRegistryFromEnvUnset checks that an unset UMONO_REGISTRY selects the
// default registry, umono.dev.
func TestRegistryFromEnvUnset(t *testing.T) {
	t.Setenv("UMONO_REGISTRY", "x")
	if err := os.Unsetenv("UMONO_REGISTRY"); err != nil {
		t.Fatalf("os.Unsetenv(UMONO_REGISTRY): %v", err)
	}
	r, err := RegistryFromEnv()
	if err != nil {
		t.Fatalf("RegistryFromEnv() err = %v, want nil", err)
	}
	if r.BaseURL != "https://umono.dev" {
		t.Fatalf("r.BaseURL = %q, want %q", r.BaseURL, "https://umono.dev")
	}
}

// TestRegistryFromEnvMirror checks that a set UMONO_REGISTRY replaces umono.dev
// entirely.
func TestRegistryFromEnvMirror(t *testing.T) {
	t.Setenv("UMONO_REGISTRY", "http://localhost:8080/")
	r, err := RegistryFromEnv()
	if err != nil {
		t.Fatalf("RegistryFromEnv() err = %v, want nil", err)
	}
	if r.BaseURL != "http://localhost:8080" {
		t.Fatalf("r.BaseURL = %q, want %q", r.BaseURL, "http://localhost:8080")
	}
	if r.Client == nil {
		t.Fatalf("r.Client = nil, want non-nil")
	}
}

// TestRegistryFromEnvInvalid checks that an unusable UMONO_REGISTRY is
// rejected.
func TestRegistryFromEnvInvalid(t *testing.T) {
	t.Setenv("UMONO_REGISTRY", "ftp://localhost")
	_, err := RegistryFromEnv()
	if !errors.Is(err, ErrRegistryURL) {
		t.Fatalf("RegistryFromEnv() err = %v, want ErrRegistryURL", err)
	}
}

// TestRegistryErrorsNameTheHost checks that the errors of a fetch name the
// host of the registry they came from, not umono.dev.
func TestRegistryErrorsNameTheHost(t *testing.T) {
	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(notFound.Close)
	r := Registry{BaseURL: notFound.URL, Client: notFound.Client()}
	host := strings.TrimPrefix(notFound.URL, "http://")
	if r.Host() != host {
		t.Fatalf("r.Host() = %q, want %q", r.Host(), host)
	}
	_, err := EnsureContract(context.Background(), t.TempDir(), "umono/page@v0.1", r)
	if err == nil {
		t.Fatalf("EnsureContract() err = nil, want error")
	}
	if !strings.Contains(err.Error(), r.Host()) {
		t.Fatalf("EnsureContract() err = %q, want it to contain %q", err, r.Host())
	}
	if strings.Contains(err.Error(), "umono.dev") {
		t.Fatalf("EnsureContract() err = %q, want it to not contain %q", err, "umono.dev")
	}

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)
	r = Registry{BaseURL: failing.URL, Client: failing.Client()}
	host = strings.TrimPrefix(failing.URL, "http://")
	_, err = EnsureContract(context.Background(), t.TempDir(), "umono/page@v0.1", r)
	if !errors.Is(err, ErrRegistryUnreachable) {
		t.Fatalf("EnsureContract() err = %v, want ErrRegistryUnreachable", err)
	}
	if !strings.Contains(err.Error(), host) {
		t.Fatalf("EnsureContract() err = %q, want it to contain %q", err, host)
	}
	if strings.Contains(err.Error(), "umono.dev") {
		t.Fatalf("EnsureContract() err = %q, want it to not contain %q", err, "umono.dev")
	}
}

// TestEnsureContractVerifiesSignature checks that EnsureContract fetches the
// contract and the signature next to it, verifies the signature against the
// contract and writes the contract to the cache when the signature is good.
func TestEnsureContractVerifiesSignature(t *testing.T) {
	contract := readTestdata(t, "page-v0.1.toml")
	signature := readTestdata(t, "page-v0.1.toml.minisig")
	var signatureRequests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry/v1/contracts/umono/page/v0.1":
			_, _ = w.Write(contract)
		case "/registry/v1/contracts/umono/page/v0.1.minisig":
			signatureRequests.Add(1)
			_, _ = w.Write(signature)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	root := t.TempDir()
	r := Registry{BaseURL: srv.URL, Client: srv.Client()}
	cachePath := filepath.Join(root, ".dev", "contracts", "umono", "page", "v0.1.toml")

	path, err := EnsureContract(context.Background(), root, "umono/page@v0.1", r)
	if err != nil {
		t.Fatalf("EnsureContract() err = %v, want nil", err)
	}
	if path != cachePath {
		t.Fatalf("path = %q, want %q", path, cachePath)
	}
	got, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if !bytes.Equal(got, contract) {
		t.Fatalf("cache contents = %q, want %q", got, contract)
	}
	if signatureRequests.Load() == 0 {
		t.Fatalf("signature requests = %d, want at least 1", signatureRequests.Load())
	}
}

// TestEnsureContractBadSignature checks that EnsureContract refuses a
// contract whose signature does not verify: the error names the registry, it
// is not a network error, and neither the contract nor its directory is
// written to the cache.
func TestEnsureContractBadSignature(t *testing.T) {
	contract := readTestdata(t, "page-v0.1.toml")
	signature := corruptSignature(t, readTestdata(t, "page-v0.1.toml.minisig"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry/v1/contracts/umono/page/v0.1":
			_, _ = w.Write(contract)
		case "/registry/v1/contracts/umono/page/v0.1.minisig":
			_, _ = w.Write(signature)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	root := t.TempDir()
	r := Registry{BaseURL: srv.URL, Client: srv.Client()}
	cachePath := filepath.Join(root, ".dev", "contracts", "umono", "page", "v0.1.toml")

	_, err := EnsureContract(context.Background(), root, "umono/page@v0.1", r)
	if err == nil {
		t.Fatalf("EnsureContract() err = nil, want error")
	}
	if !errors.Is(err, ErrContractNotVerified) {
		t.Fatalf("EnsureContract() err = %v, want ErrContractNotVerified", err)
	}
	if !errors.Is(err, themecore.ErrContractSignature) {
		t.Fatalf("EnsureContract() err = %v, want themecore.ErrContractSignature", err)
	}
	if errors.Is(err, ErrRegistryUnreachable) {
		t.Fatalf("EnsureContract() err = %v, want it to not be ErrRegistryUnreachable", err)
	}
	if !strings.Contains(err.Error(), r.Host()) {
		t.Fatalf("EnsureContract() err = %q, want it to contain %q", err, r.Host())
	}
	if _, err := os.Stat(cachePath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(%s) err = %v, want fs.ErrNotExist", cachePath, err)
	}
	devDir := filepath.Join(root, ".dev")
	if _, err := os.Stat(devDir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(%s) err = %v, want fs.ErrNotExist", devDir, err)
	}
}

// TestEnsureContractSignatureNotFound checks that EnsureContract reports a
// contract the registry serves without a published signature as not published
// and writes nothing to the cache.
func TestEnsureContractSignatureNotFound(t *testing.T) {
	contract := readTestdata(t, "page-v0.1.toml")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry/v1/contracts/umono/page/v0.1":
			_, _ = w.Write(contract)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	root := t.TempDir()
	r := Registry{BaseURL: srv.URL, Client: srv.Client()}
	cachePath := filepath.Join(root, ".dev", "contracts", "umono", "page", "v0.1.toml")

	_, err := EnsureContract(context.Background(), root, "umono/page@v0.1", r)
	if !errors.Is(err, ErrContractNotPublished) {
		t.Fatalf("EnsureContract() err = %v, want ErrContractNotPublished", err)
	}
	if _, err := os.Stat(cachePath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Stat(%s) err = %v, want fs.ErrNotExist", cachePath, err)
	}
}

// TestEnsureContractCachedIsNotVerified checks that EnsureContract returns a
// cached contract as it is: it never fetches it again, so a contract that was
// never verified stays in the cache.
func TestEnsureContractCachedIsNotVerified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s", r.URL.Path)
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	root := t.TempDir()
	cachePath := filepath.Join(root, ".dev", "contracts", "umono", "page", "v0.1.toml")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("cached"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	r := Registry{BaseURL: srv.URL, Client: srv.Client()}
	path, err := EnsureContract(context.Background(), root, "umono/page@v0.1", r)
	if err != nil {
		t.Fatalf("EnsureContract() err = %v, want nil", err)
	}
	if path != cachePath {
		t.Fatalf("path = %q, want %q", path, cachePath)
	}
	got, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	if string(got) != "cached" {
		t.Fatalf("cache contents = %q, want %q", got, "cached")
	}
}

// corruptSignature returns sig with one character of its last line, the
// signature over the trusted comment, changed, so that the signature does not
// verify against the contract anymore.
func corruptSignature(t *testing.T, sig []byte) []byte {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(string(sig), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("signature has %d lines, want 4", len(lines))
	}
	last := lines[len(lines)-1]
	data := strings.TrimRight(last, "=")
	if len(data) == 0 {
		t.Fatalf("last line of the signature is empty")
	}
	i := len(data) / 2
	if data[i] == 'A' {
		last = last[:i] + "B" + last[i+1:]
	} else {
		last = last[:i] + "A" + last[i+1:]
	}
	lines[len(lines)-1] = last
	return []byte(strings.Join(lines, "\n") + "\n")
}
