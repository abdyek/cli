package compatibility

import "testing"

func TestIsVersionCompatible(t *testing.T) {
	tests := []struct {
		name       string
		cliVersion string
		minVersion string
		want       bool
	}{
		{"equal versions", "0.1.0", "0.1.0", true},
		{"equal with v prefix", "v0.1.0", "0.1.0", true},

		{"cli major newer", "1.0.0", "0.1.0", true},
		{"cli minor newer", "0.2.0", "0.1.0", true},
		{"cli patch newer", "0.1.1", "0.1.0", true},
		{"cli much newer", "2.5.3", "0.1.0", true},

		{"cli major older", "0.1.0", "1.0.0", false},
		{"cli minor older", "0.1.0", "0.2.0", false},
		{"cli patch older", "0.1.0", "0.1.1", false},
		{"cli much older", "0.1.0", "2.5.3", false},

		{"min version 0.0.0", "0.1.0", "0.0.0", true},
		{"both 0.0.0", "0.0.0", "0.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isVersionCompatible(tt.cliVersion, tt.minVersion)
			if got != tt.want {
				t.Errorf("isVersionCompatible(%q, %q) = %v, want %v",
					tt.cliVersion, tt.minVersion, got, tt.want)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
	}{
		{"0.1.0", [3]int{0, 1, 0}},
		{"v0.1.0", [3]int{0, 1, 0}},
		{"1.2.3", [3]int{1, 2, 3}},
		{"v1.2.3", [3]int{1, 2, 3}},
		{"0.1", [3]int{0, 1, 0}},
		{"1", [3]int{1, 0, 0}},
		{"1.0.0-beta", [3]int{1, 0, 0}},
		{"2.1.3-rc1", [3]int{2, 1, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseVersion(tt.input)
			if got != tt.want {
				t.Errorf("parseVersion(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
