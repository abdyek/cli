// Package downloadtest serves a fake GitHub API with Umono releases, so the
// tests of the commands that download Umono do not go to GitHub.
package downloadtest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Server is a fake GitHub API of umono-cms/umono with one release, which is
// also the latest.
type Server struct {
	*httptest.Server
	// Tag is the release's tag, for example "v0.9.0".
	Tag string
}

// New starts a fake GitHub API whose latest release tag has three assets:
// the archive of files for this platform (runtime.GOOS and runtime.GOARCH),
// checksums.txt with the archive's SHA-256, and umono.json, the file of that
// name in files ("{}" without one). The server is closed at the end of the
// test.
func New(t testing.TB, tag string, files map[string][]byte) *Server {
	t.Helper()

	osName, archName := platformNames()
	archiveName := "umono_" + strings.TrimPrefix(tag, "v") + "_" + osName + "_" + archName + ".tar.gz"

	archiveBytes, err := buildArchive(files)
	if err != nil {
		t.Fatalf("downloadtest: build archive: %v", err)
	}

	sum := sha256.Sum256(archiveBytes)
	checksumsData := []byte(hex.EncodeToString(sum[:]) + "  " + archiveName + "\n")

	umonoJSON, ok := files["umono.json"]
	if !ok {
		umonoJSON = []byte("{}")
	}

	mux := http.NewServeMux()
	srv := &Server{
		Server: httptest.NewServer(mux),
		Tag:    tag,
	}

	releaseBody, err := buildReleaseJSON(srv.URL, tag, archiveName, int64(len(archiveBytes)))
	if err != nil {
		t.Fatalf("downloadtest: build release JSON: %v", err)
	}

	sendRelease := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(releaseBody)
	}
	mux.HandleFunc("/repos/umono-cms/umono/releases/latest", sendRelease)
	mux.HandleFunc("/repos/umono-cms/umono/releases/tags/"+tag, sendRelease)

	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		switch strings.TrimPrefix(r.URL.Path, "/download/") {
		case archiveName:
			_, _ = w.Write(archiveBytes)
		case "checksums.txt":
			_, _ = w.Write(checksumsData)
		case "umono.json":
			_, _ = w.Write(umonoJSON)
		default:
			http.NotFound(w, r)
		}
	})

	t.Cleanup(func() {
		srv.Close()
	})

	return srv
}

func platformNames() (string, string) {
	osMap := map[string]string{
		"linux":  "Linux",
		"darwin": "Darwin",
	}
	archMap := map[string]string{
		"amd64": "x86_64",
		"arm64": "arm64",
	}

	osName, ok := osMap[runtime.GOOS]
	if !ok {
		osName = runtime.GOOS
	}

	archName, ok := archMap[runtime.GOARCH]
	if !ok {
		archName = runtime.GOARCH
	}

	return osName, archName
}

func buildArchive(files map[string][]byte) ([]byte, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for _, name := range names {
		mode := int64(0o644)
		if name == "umono" {
			mode = 0o755
		}
		hdr := &tar.Header{
			Name:     name,
			Mode:     mode,
			Size:     int64(len(files[name])),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(files[name]); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func buildReleaseJSON(serverURL, tag, archiveName string, archiveSize int64) ([]byte, error) {
	type asset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size,omitempty"`
	}
	type release struct {
		TagName string  `json:"tag_name"`
		Assets  []asset `json:"assets"`
	}

	return json.Marshal(release{
		TagName: tag,
		Assets: []asset{
			{Name: archiveName, BrowserDownloadURL: serverURL + "/download/" + archiveName, Size: archiveSize},
			{Name: "checksums.txt", BrowserDownloadURL: serverURL + "/download/checksums.txt"},
			{Name: "umono.json", BrowserDownloadURL: serverURL + "/download/umono.json"},
		},
	})
}
