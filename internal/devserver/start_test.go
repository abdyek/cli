package devserver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestMain runs the test binary as a fake Umono when UMONO_DEVSERVER_FAKE is
// set: it listens on 127.0.0.1:$PORT and answers every request with
// $UMONO_THEME_DEV, a newline and its working directory.
func TestMain(m *testing.M) {
	if os.Getenv("UMONO_DEVSERVER_FAKE") == "1" {
		wd, _ := os.Getwd()
		body := os.Getenv("UMONO_THEME_DEV") + "\n" + wd
		err := http.ListenAndServe("127.0.0.1:"+os.Getenv("PORT"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, body)
		}))
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// writeFakeBinary writes Dir(project)/umono as a script that runs the test
// binary as a fake Umono.
func writeFakeBinary(t *testing.T, project string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	dir := Dir(project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	path := filepath.Join(dir, "umono")
	content := "#!/bin/sh\nUMONO_DEVSERVER_FAKE=1 exec '" + exe + "'\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestStartRunsTheServerInDevMode(t *testing.T) {
	project := t.TempDir()
	writeFakeBinary(t, project)

	s, err := Start(project)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Fatalf("Stop: %v", err)
		}
	})

	resp, err := http.Get(s.URL)
	if err != nil {
		t.Fatalf("http.Get(%s): %v", s.URL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	got := string(body)
	want := project + "\n" + Dir(project)
	if got != want {
		parts := strings.SplitN(got, "\n", 2)
		wantParts := strings.SplitN(want, "\n", 2)
		if len(parts) == 2 && len(wantParts) == 2 {
			match := true
			for i := range parts {
				g, errG := filepath.EvalSymlinks(parts[i])
				w, errW := filepath.EvalSymlinks(wantParts[i])
				if errG != nil || errW != nil || g != w {
					match = false
					break
				}
			}
			if !match {
				t.Fatalf("body = %q, want %q", got, want)
			}
		} else {
			t.Fatalf("body = %q, want %q", got, want)
		}
	}

	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, err := http.Get(s.URL); err == nil {
		t.Fatalf("http.Get(%s) after Stop: nil error, want non-nil", s.URL)
	}
}

func TestStartFailsWhenTheServerExits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	project := t.TempDir()
	dir := Dir(project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	path := filepath.Join(dir, "umono")
	content := "#!/bin/sh\necho broken >&2\nexit 1\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}

	_, err := Start(project)
	if err == nil {
		t.Fatalf("Start returned nil error, want non-nil")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Fatalf("err = %v, want it to contain %q", err, "broken")
	}
}
