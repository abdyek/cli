// Package devwatch watches the files of a theme project that umono theme
// dev follows. It polls, so it needs no file system notifications.
package devwatch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// stamp is what tells a file's change: its size and modification time.
type stamp struct {
	size int64
	mod  time.Time
}

// Snapshot is the stamp of theme.toml, attributes.toml and every file under
// assets/, by slash path. A missing file is left out.
type Snapshot map[string]stamp

// Scan takes the snapshot of projectDir.
func Scan(projectDir string) (Snapshot, error) {
	s := Snapshot{}
	for _, name := range []string{"theme.toml", "attributes.toml"} {
		info, err := os.Stat(filepath.Join(projectDir, name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		s[name] = stamp{info.Size(), info.ModTime()}
	}
	root := filepath.Join(projectDir, "assets")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) && path == root {
			return filepath.SkipDir
		}
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(projectDir, path)
		if err != nil {
			return err
		}
		s[filepath.ToSlash(rel)] = stamp{info.Size(), info.ModTime()}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Changed returns the paths whose file was added, removed or changed from
// old to s, sorted.
func (s Snapshot) Changed(old Snapshot) []string {
	var changed []string
	for path, st := range s {
		if prev, ok := old[path]; !ok || prev.size != st.size || !prev.mod.Equal(st.mod) {
			changed = append(changed, path)
		}
	}
	for path := range old {
		if _, ok := s[path]; !ok {
			changed = append(changed, path)
		}
	}
	slices.Sort(changed)
	return changed
}

// OnlyCSS reports whether paths is just assets/theme.css: the page then
// swaps its stylesheet instead of reloading.
func OnlyCSS(paths []string) bool {
	return len(paths) == 1 && paths[0] == "assets/theme.css"
}

// Watch scans projectDir every interval and calls fn with the changed paths,
// until ctx is done. A scan that fails is tried again at the next interval.
func Watch(ctx context.Context, projectDir string, interval time.Duration, fn func(changed []string)) {
	last, _ := Scan(projectDir)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		s, err := Scan(projectDir)
		if err != nil {
			continue
		}
		if changed := s.Changed(last); len(changed) > 0 {
			last = s
			fn(changed)
		}
	}
}
