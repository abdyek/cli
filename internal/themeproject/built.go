package themeproject

import (
	"strconv"
	"strings"
	"time"

	"github.com/umono-cms/themecore"
)

const builtSchemaURL = "https://umono.dev/schemas/theme-built/v1"

// RenderBuilt returns built.toml: the values of theme.toml unchanged, with
// the detected theme-api-version and the build time in UTC.
func RenderBuilt(m themecore.Manifest, version int, at time.Time) []byte {
	var b strings.Builder
	line := func(key, value string) {
		b.WriteString(key + " = " + value + "\n")
	}
	optional := func(key string, value *string) {
		if value != nil {
			line(key, quote(*value))
		}
	}
	line("schema", quote(builtSchemaURL))
	line("id", quote(m.ID))
	line("contract", quote(m.Contract))
	line("theme-api-version", strconv.Itoa(version))
	line("name", quote(m.Name))
	line("version", quote(m.Version))
	optional("description", m.Description)
	optional("license", m.License)
	optional("homepage", m.Homepage)
	optional("repository", m.Repository)
	line("built-at", at.UTC().Format(time.RFC3339))
	for _, a := range m.Authors {
		b.WriteString("\n[[authors]]\n")
		line("name", quote(a.Name))
		optional("url", a.URL)
	}
	return []byte(b.String())
}
