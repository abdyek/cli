package cmd

import (
	"errors"
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

func TestThemeSubcommandsAreNotImplementedYet(t *testing.T) {
	for _, name := range []string{"dev", "install"} {
		sub, _, err := rootCmd.Find([]string{"theme", name})
		if err != nil {
			t.Fatalf("rootCmd.Find([\"theme\", %q]) returned error: %v", name, err)
		}
		if got, want := sub.Name(), name; got != want {
			t.Fatalf("sub.Name() = %q, want %q", got, want)
		}

		err = sub.RunE(sub, nil)
		if !errors.Is(err, ErrNotImplemented) {
			t.Fatalf("sub.RunE(sub, nil) for %q returned %v, want ErrNotImplemented", name, err)
		}
	}
}
