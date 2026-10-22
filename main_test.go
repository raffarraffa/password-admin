package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePortFromEnv(t *testing.T) {
	got := parsePortFromEnv([]byte("# server port\nHOST=localhost\nexport PORT = '8123'\nPORT=9000\n"))
	if got != "9000" {
		t.Fatalf("parsePortFromEnv() = %q, want last PORT value %q", got, "9000")
	}
}

func TestConfiguredPortUsesEnvironment(t *testing.T) {
	t.Setenv("PORT", "8123")
	got, err := configuredPort()
	if err != nil {
		t.Fatal(err)
	}
	if got != "8123" {
		t.Fatalf("configuredPort() = %q, want %q", got, "8123")
	}
}

func TestConfiguredPortReadsDotEnvFromWorkingDirectory(t *testing.T) {
	t.Setenv("PORT", "")
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, ".env"), []byte("PORT=8124\n"), 0600); err != nil {
		t.Fatal(err)
	}
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	got, err := configuredPort()
	if err != nil {
		t.Fatal(err)
	}
	if got != "8124" {
		t.Fatalf("configuredPort() = %q, want %q", got, "8124")
	}
}

func TestConfiguredPortRejectsInvalidValue(t *testing.T) {
	t.Setenv("PORT", "70000")
	if _, err := configuredPort(); err == nil {
		t.Fatal("configuredPort() accepted a port outside the valid range")
	}
}
