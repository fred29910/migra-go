package version

import (
	"strings"
	"testing"
)

func TestInfo_ContainsAllFields(t *testing.T) {
	Version = "v0.2.0"
	BuildTime = "2026-05-14T10:00:00Z"
	GitCommit = "abc1234"
	GoVersion = "go1.26.2"
	defer func() {
		Version = "dev"
		BuildTime = "unknown"
		GitCommit = "unknown"
		GoVersion = "unknown"
	}()

	got := Info()
	for _, field := range []string{"Version:", "Built:", "Git Commit:", "Go Version:"} {
		if !strings.Contains(got, field) {
			t.Errorf("Info() missing field %q, got:\n%s", field, got)
		}
	}
	if !strings.Contains(got, "v0.2.0") {
		t.Errorf("Info() should contain version, got:\n%s", got)
	}
}

func TestShort_ReturnsVersion(t *testing.T) {
	Version = "v0.2.0"
	defer func() { Version = "dev" }()

	got := Short()
	if got != "v0.2.0" {
		t.Errorf("Short() = %q, want %q", got, "v0.2.0")
	}
}

func TestDefaults(t *testing.T) {
	Version = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
	GoVersion = "unknown"

	got := Info()
	if !strings.Contains(got, "dev") {
		t.Errorf("Info() with defaults should contain 'dev', got:\n%s", got)
	}
	if Short() != "dev" {
		t.Errorf("Short() with defaults = %q, want %q", Short(), "dev")
	}
}
