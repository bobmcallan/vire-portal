package config

import "testing"

func TestGetVersion(t *testing.T) {
	// Default should be "dev"
	v := GetVersion()
	if v != "dev" {
		t.Errorf("expected default version dev, got %s", v)
	}
}

func TestGetBuild(t *testing.T) {
	b := GetBuild()
	if b != "unknown" {
		t.Errorf("expected default build unknown, got %s", b)
	}
}

func TestGetGitCommit(t *testing.T) {
	gc := GetGitCommit()
	if gc != "unknown" {
		t.Errorf("expected default git commit unknown, got %s", gc)
	}
}

func TestGetFullVersion(t *testing.T) {
	fv := GetFullVersion()
	expected := "dev (build: unknown, commit: unknown)"
	if fv != expected {
		t.Errorf("expected full version %q, got %q", expected, fv)
	}
}
