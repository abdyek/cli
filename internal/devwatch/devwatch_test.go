package devwatch

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func writeFile(t *testing.T, path string, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestChanged(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "theme.toml"), "a")
	writeFile(t, filepath.Join(dir, "assets", "theme.css"), "a")
	writeFile(t, filepath.Join(dir, "README.md"), "a")

	s1, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	writeFile(t, filepath.Join(dir, "assets", "theme.css"), "bb")
	writeFile(t, filepath.Join(dir, "README.md"), "bb")
	s2, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	got := s2.Changed(s1)
	want := []string{"assets/theme.css"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	writeFile(t, filepath.Join(dir, "assets", "fonts", "a.woff2"), "a")
	writeFile(t, filepath.Join(dir, "attributes.toml"), "a")
	s3, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	got = s3.Changed(s2)
	want = []string{"assets/fonts/a.woff2", "attributes.toml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	if err := os.Remove(filepath.Join(dir, "assets", "theme.css")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	s4, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	got = s4.Changed(s3)
	want = []string{"assets/theme.css"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	if got := s4.Changed(s4); len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}

func TestScanWithoutAssets(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "theme.toml"), "a")
	s, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(s) != 1 {
		t.Fatalf("len %d, want 1", len(s))
	}
}

func TestOnlyCSS(t *testing.T) {
	tests := []struct {
		paths []string
		want  bool
	}{
		{[]string{"assets/theme.css"}, true},
		{[]string{"assets/theme.css", "theme.toml"}, false},
		{[]string{"assets/logo.png"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := OnlyCSS(tt.paths); got != tt.want {
			t.Errorf("OnlyCSS(%v) = %v, want %v", tt.paths, got, tt.want)
		}
	}
}

func TestWatchCallsOnAChange(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "theme.toml"), "a")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes := make(chan []string, 1)
	done := make(chan struct{})
	go func() {
		Watch(ctx, dir, 10*time.Millisecond, func(c []string) {
			select {
			case changes <- c:
			default:
			}
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	writeFile(t, filepath.Join(dir, "theme.toml"), "bb")

	select {
	case c := <-changes:
		want := []string{"theme.toml"}
		if !reflect.DeepEqual(c, want) {
			t.Fatalf("got %v, want %v", c, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no change")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not return")
	}
}
