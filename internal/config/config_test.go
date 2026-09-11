package config

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
}

func write(t *testing.T, body string) {
	t.Helper()
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(File(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// extra_generated_paths extends the built-in list rather than replacing it.
func TestExtraPathsExtendTheDefaults(t *testing.T) {
	isolate(t)
	write(t, "extra_generated_paths = ['golden-sessions/']\n")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"pnpm-lock.yaml", ".terraform.lock.hcl", "golden-sessions/"} {
		if !slices.Contains(c.GeneratedPaths, want) {
			t.Errorf("effective generated paths lack %q", want)
		}
	}
}

// generated_paths, when set, still replaces the built-in list entirely, so a
// config written before extra_generated_paths existed keeps meaning the same.
func TestGeneratedPathsReplaceTheDefaults(t *testing.T) {
	isolate(t)
	write(t, "generated_paths = ['only/']\nextra_generated_paths = ['also/']\n")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(c.GeneratedPaths, ","); got != "only/,also/" {
		t.Errorf("generated paths = %q, want only/,also/", got)
	}
}

// Save must not copy the built-in lists into the file: a copied list freezes
// them, and the user never receives improved defaults on upgrade.
func TestSaveWritesOnlyTheUsersChoices(t *testing.T) {
	isolate(t)
	if err := (&Config{Author: "someone", Orgs: []string{"acme"}}).Save(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(File())
	if err != nil {
		t.Fatal(err)
	}
	for _, frozen := range []string{"generated_paths", "generated_suffixes", "big_file_lines", "window_days"} {
		if strings.Contains(string(b), frozen) {
			t.Errorf("saved config contains %q, which would freeze the default:\n%s", frozen, b)
		}
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Author != "someone" || !slices.Equal(c.Orgs, []string{"acme"}) || !slices.Contains(c.GeneratedPaths, "pnpm-lock.yaml") {
		t.Errorf("round trip lost a value: author=%q orgs=%v paths=%d", c.Author, c.Orgs, len(c.GeneratedPaths))
	}
}
