package themeproject

import (
	"fmt"
	"strings"

	"github.com/umono-cms/themecore"
)

const schemaURL = "https://umono.dev/schemas/theme-manifest/v1"

// The two runes a TOML basic string escapes with a backslash.
const (
	dquote    = rune(34)
	backslash = rune(92)
)

// Manifest holds the answers of umono theme create. Empty optional fields are
// not written. AuthorName and AuthorURL make the single [[authors]] entry.
type Manifest struct {
	ID          string
	Name        string
	Contract    string
	Version     string
	Description string
	License     string
	Homepage    string
	Repository  string
	AuthorName  string
	AuthorURL   string
}

// Render returns the theme.toml text of m.
func Render(m Manifest) []byte {
	var b strings.Builder
	line := func(key, value string) {
		b.WriteString(key + " = " + quote(value) + "\n")
	}
	optional := func(key, value string) {
		if value != "" {
			line(key, value)
		}
	}
	line("schema", schemaURL)
	line("id", m.ID)
	line("contract", m.Contract)
	line("name", m.Name)
	line("version", m.Version)
	optional("description", m.Description)
	optional("license", m.License)
	optional("homepage", m.Homepage)
	optional("repository", m.Repository)
	if m.AuthorName != "" || m.AuthorURL != "" {
		b.WriteString("\n[[authors]]\n")
		line("name", m.AuthorName)
		optional("url", m.AuthorURL)
	}
	return []byte(b.String())
}

// Validate checks m with the rules of theme.toml, so that create writes
// nothing that build would reject.
func (m Manifest) Validate() error {
	_, err := themecore.ParseManifest(Render(m))
	return err
}

// quote returns s as a TOML basic string.
func quote(s string) string {
	var b strings.Builder
	b.WriteRune(dquote)
	for _, r := range s {
		switch {
		case r == dquote || r == backslash:
			b.WriteRune(backslash)
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			b.WriteRune(backslash)
			fmt.Fprintf(&b, "u%04X", r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteRune(dquote)
	return b.String()
}
