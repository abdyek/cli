package themedev

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/umono-cms/cli/internal/download"
	"github.com/umono-cms/cli/internal/download/downloadtest"
	"github.com/umono-cms/cli/internal/themeproject"
	"github.com/umono-cms/themecore"
)

// TestMain runs the test binary as a fake Umono of .dev/server when
// UMONO_THEMEDEV_FAKE is set: it listens on 127.0.0.1:$PORT and answers /
// with an HTML page whose CSP has the nonce n1 and whose body is
// $UMONO_THEME_DEV.
func TestMain(m *testing.M) {
	if os.Getenv("UMONO_THEMEDEV_FAKE") == "1" {
		root := os.Getenv("UMONO_THEME_DEV")
		err := http.ListenAndServe("127.0.0.1:"+os.Getenv("PORT"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-n1'")
			io.WriteString(w, "<html><head></head><body>"+root+"</body></html>")
		}))
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "themeproject", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newProject(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "theme")
	writeFile(t, filepath.Join(dir, "theme.toml"), fixture(t, "minimal.toml"))
	writeFile(t, filepath.Join(dir, "assets", "theme.css"), themecore.FallbackSource())
	writeFile(t, filepath.Join(dir, ".dev", "contracts", "umono", "page", "v0.1.toml"), fixture(t, "page-v0.1.toml"))
	return dir
}

// fakeRelease returns a client of a fake GitHub API whose release runs the
// test binary as Umono.
func fakeRelease(t *testing.T) *download.Client {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fakeRelease uses a POSIX shell script")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	files := map[string][]byte{
		"umono":        []byte("#!/bin/sh\nUMONO_THEMEDEV_FAKE=1 exec '" + exe + "'\n"),
		".env.example": []byte("APP_ENV=\n"),
		"umono.json":   []byte(fmt.Sprintf(`{"min_cli_version": "0.0.0", "theme_api_current": %d, "theme_api_min": %d}`, themecore.ThemeAPICurrent(), themecore.ThemeAPIMin())),
	}
	srv := downloadtest.New(t, "v0.9.0", files)
	client, err := download.NewClientWithURL(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithURL: %v", err)
	}
	return client
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

var noRegistry = themeproject.Registry{BaseURL: "http://127.0.0.1:1", Client: &http.Client{Timeout: time.Second}}

func options(project string, port int, client *download.Client) Options {
	return Options{
		ProjectDir: project,
		Port:       port,
		Client:     client,
		Registry:   noRegistry,
		Out:        io.Discard,
		Interval:   20 * time.Millisecond,
	}
}

func TestRunServesThePagesThroughTheProxy(t *testing.T) {
	project := newProject(t)
	port := freePort(t)
	client := fakeRelease(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errc := make(chan error, 1)
	go func() {
		errc <- Run(ctx, options(project, port, client))
	}()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	get := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(15 * time.Second)
	var body string
	for time.Now().Before(deadline) {
		select {
		case err := <-errc:
			t.Fatalf("Run returned early: %v", err)
		default:
		}
		resp, err := get.Get(base + "/")
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			body = string(b)
			break
		}
		resp.Body.Close()
		time.Sleep(100 * time.Millisecond)
	}
	if body == "" {
		t.Fatal("server did not respond with 200")
	}

	wantScript := `<script nonce="n1" src="/_umono-dev/client.js" defer></script>`
	if !strings.Contains(body, wantScript) {
		t.Fatalf("body missing %q\nbody: %s", wantScript, body)
	}
	if !strings.Contains(body, project) {
		t.Fatalf("body missing project %q\nbody: %s", project, body)
	}

	builtPath := filepath.Join(project, ".dev", "server", "themes", "dev", "dev", "0.0.0", "built.toml")
	if _, err := os.Stat(builtPath); err != nil {
		t.Fatalf("stat %s: %v", builtPath, err)
	}

	streamCtx, streamCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer streamCancel()
	req, err := http.NewRequestWithContext(streamCtx, http.MethodGet, base+"/_umono-dev/events", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)

	nextEventName := func() (string, error) {
		for {
			var name string
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					return "", err
				}
				line = strings.TrimRight(line, "\r\n")
				if line == "" {
					break
				}
				if strings.HasPrefix(line, ":") {
					continue
				}
				if strings.HasPrefix(line, "event: ") {
					name = strings.TrimPrefix(line, "event: ")
				}
			}
			if name != "" {
				return name, nil
			}
		}
	}

	if name, err := nextEventName(); err != nil {
		t.Fatalf("first event: %v", err)
	} else if name != "state" {
		t.Fatalf("first event = %q, want %q", name, "state")
	}

	writeFile(t, filepath.Join(project, "assets", "theme.css"), []byte(":scope { color: red; }"))
	if name, err := nextEventName(); err != nil {
		t.Fatalf("css event: %v", err)
	} else if name != "css" {
		t.Fatalf("css event = %q, want %q", name, "css")
	}

	writeFile(t, filepath.Join(project, "assets", "theme.css"), []byte("html { color: red }"))
	if name, err := nextEventName(); err != nil {
		t.Fatalf("error event: %v", err)
	} else if name != "error" {
		t.Fatalf("error event = %q, want %q", name, "error")
	}

	cancel()
	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not return")
	}
}

func TestRunStopsWithoutTheContract(t *testing.T) {
	project := newProject(t)
	if err := os.RemoveAll(filepath.Join(project, ".dev", "contracts")); err != nil {
		t.Fatalf("remove contracts: %v", err)
	}

	err := Run(context.Background(), options(project, freePort(t), nil))
	if !errors.Is(err, themeproject.ErrRegistryUnreachable) {
		t.Fatalf("err = %v, want themeproject.ErrRegistryUnreachable", err)
	}

	if _, err := os.Stat(filepath.Join(project, ".dev", "server")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("stat .dev/server: err = %v, want fs.ErrNotExist", err)
	}
}

func TestRunStopsOnABusyPort(t *testing.T) {
	project := newProject(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	err = Run(context.Background(), options(project, port, fakeRelease(t)))
	if err == nil {
		t.Fatal("err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "--port") {
		t.Fatalf("err = %v, want it to contain %q", err, "--port")
	}
}

func TestRunStopsOutsideAThemeProject(t *testing.T) {
	err := Run(context.Background(), options(t.TempDir(), freePort(t), nil))
	if !errors.Is(err, themeproject.ErrNotThemeProject) {
		t.Fatalf("err = %v, want themeproject.ErrNotThemeProject", err)
	}
}
