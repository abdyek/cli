package themeproject

import (
	"testing"

	"github.com/umono-cms/themecore"
)

// TestThemecoreDetectorCorpus runs themecore's detector corpus with the
// Compono version go.mod selects for the CLI. A Compono bump that changes the
// signals the detectors read fails it.
func TestThemecoreDetectorCorpus(t *testing.T) {
	if err := themecore.DetectorCorpus(); err != nil {
		t.Fatal(err)
	}
}

// TestThemecoreTOMLConformance checks that the TOML decoder the CLI links
// rejects every input outside TOML v1.0.0. Theme files are TOML v1.0.0, and
// the decoder version Go selects for the CLI can differ from themecore's.
func TestThemecoreTOMLConformance(t *testing.T) {
	if err := themecore.TOMLConformance(); err != nil {
		t.Fatal(err)
	}
}
