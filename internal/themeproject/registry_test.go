package themeproject

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
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
