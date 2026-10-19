package cmd

import (
	"slices"
	"testing"
)

func TestThemeCommandGroup(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"theme"})
	if err != nil {
		t.Fatalf("rootCmd.Find([\"theme\"]) returned error: %v", err)
	}
	if got, want := cmd.Name(), "theme"; got != want {
		t.Fatalf("cmd.Name() = %q, want %q", got, want)
	}

	names := make([]string, 0, len(cmd.Commands()))
	for _, c := range cmd.Commands() {
		names = append(names, c.Name())
	}
	slices.Sort(names)

	want := []string{"build", "create", "dev", "install"}
	if !slices.Equal(names, want) {
		t.Fatalf("cmd.Commands() names = %v, want %v", names, want)
	}
}

func TestThemeDevFlags(t *testing.T) {
	sub, _, err := rootCmd.Find([]string{"theme", "dev"})
	if err != nil {
		t.Fatalf("rootCmd.Find([\"theme\", \"dev\"]) returned error: %v", err)
	}

	portFlag := sub.Flags().Lookup("port")
	if portFlag == nil {
		t.Fatalf("sub.Flags().Lookup(\"port\") returned nil")
	}
	if got, want := portFlag.DefValue, "8990"; got != want {
		t.Fatalf("port DefValue = %q, want %q", got, want)
	}

	updateServerFlag := sub.Flags().Lookup("update-server")
	if updateServerFlag == nil {
		t.Fatalf("sub.Flags().Lookup(\"update-server\") returned nil")
	}
	if got, want := updateServerFlag.DefValue, "false"; got != want {
		t.Fatalf("update-server DefValue = %q, want %q", got, want)
	}
}
