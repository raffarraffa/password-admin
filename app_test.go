package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveVaultPathUsesProjectRootInDevelopment(t *testing.T) {
	projectDir := t.TempDir()
	executableDir := filepath.Join(projectDir, "build", "bin")
	if err := os.MkdirAll(executableDir, 0700); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(executableDir, "vault.sqlite")
	if err := os.WriteFile(legacyPath, []byte("existing vault"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath+"-wal", []byte("wal data"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := resolveVaultPath(filepath.Join(executableDir, "Password Admin-dev.exe"), projectDir, true)
	if err != nil {
		t.Fatalf("resolveVaultPath() error = %v", err)
	}
	want := filepath.Join(projectDir, "vault.sqlite")
	if got != want {
		t.Fatalf("resolveVaultPath() = %q, want %q", got, want)
	}
	for _, file := range []struct {
		source, destination, expected string
	}{
		{legacyPath, want, "existing vault"},
		{legacyPath + "-wal", want + "-wal", "wal data"},
	} {
		if _, err := os.Stat(file.source); !os.IsNotExist(err) {
			t.Fatalf("legacy file %q still exists or stat failed: %v", file.source, err)
		}
		data, err := os.ReadFile(file.destination)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(data) != file.expected {
			t.Fatalf("moved file contents = %q, want %q", data, file.expected)
		}
	}
}

func TestResolveVaultPathUsesExecutableDirectoryOutsideDevelopment(t *testing.T) {
	projectDir := t.TempDir()
	executableDir := filepath.Join(projectDir, "build", "bin")
	got, err := resolveVaultPath(filepath.Join(executableDir, "Password Admin.exe"), projectDir, false)
	if err != nil {
		t.Fatalf("resolveVaultPath() error = %v", err)
	}
	want := filepath.Join(executableDir, "vault.sqlite")
	if got != want {
		t.Fatalf("resolveVaultPath() = %q, want %q", got, want)
	}
}
