package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestVersionFlag_Registered(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("version")
	if f == nil {
		t.Fatal("--version flag not registered on rootCmd")
	}
	if f.Shorthand != "v" {
		t.Errorf("--version shorthand = %q, want %q", f.Shorthand, "v")
	}
}

func TestVerboseFlag_ShorthandChangedToUppercase(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("verbose")
	if f == nil {
		t.Fatal("--verbose flag not registered on rootCmd")
	}
	if f.Shorthand != "V" {
		t.Errorf("--verbose shorthand = %q, want %q", f.Shorthand, "V")
	}
}

func TestVersionCLI_Output(t *testing.T) {
	cmd := exec.Command("go", "build", "--trimpath", "-ldflags=-s -w",
		"-o", "/tmp/migra-test", "./cmd/migra")
	cmd.Dir = "../../"
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build migra: %v\n%s", err, out)
	}
	defer func() { _ = os.Remove("/tmp/migra-test") }()

	cmd = exec.Command("/tmp/migra-test", "--version")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version exited with error: %v\n%s", err, out)
	}
	output := string(out)
	for _, field := range []string{"Version:", "Built:", "Git Commit:", "Go Version:"} {
		if !strings.Contains(output, field) {
			t.Errorf("--version output missing %q, got:\n%s", field, output)
		}
	}

	cmd = exec.Command("/tmp/migra-test", "-v")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("-v exited with error: %v\n%s", err, out)
	}
	outputShort := string(out)
	if !strings.Contains(outputShort, "Version:") {
		t.Errorf("-v output should contain 'Version:', got:\n%s", outputShort)
	}
}
