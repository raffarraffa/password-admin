package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEnvValue(t *testing.T) {
	got := parseEnvValue([]byte("# database path\nHOST=localhost\nexport DB_PATH = './data/vault.sqlite'\nDB_PATH=/other/vault.sqlite\n"), "DB_PATH")
	if got != "/other/vault.sqlite" {
		t.Fatalf("parseEnvValue() = %q, want last DB_PATH value", got)
	}
}

func TestConfiguredDatabasePathUsesEnvironment(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "vault.sqlite")
	t.Setenv("DB_PATH", databasePath)
	got, err := configuredDatabasePath()
	if err != nil {
		t.Fatal(err)
	}
	if got != databasePath {
		t.Fatalf("configuredDatabasePath() = %q, want %q", got, databasePath)
	}
}

func TestConfiguredDatabasePathResolvesRelativeToExecutable(t *testing.T) {
	t.Setenv("DB_PATH", "./data/password-admin.sqlite")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(executable), "data", "password-admin.sqlite")

	got, err := configuredDatabasePath()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("configuredDatabasePath() = %q, want %q", got, want)
	}
}

func TestConfiguredDatabasePathDefaultsToPortableDataFolder(t *testing.T) {
	t.Setenv("DB_PATH", "")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(executable), "data", "password-admin.sqlite")

	got, err := configuredDatabasePath()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("configuredDatabasePath() = %q, want %q", got, want)
	}
}

func TestConfiguredValueReadsDotEnvFromExecutableDirectory(t *testing.T) {
	dotEnv := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(dotEnv, []byte("DB_PATH=./data/vault.sqlite\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readDotEnvValue(dotEnv, "DB_PATH")
	if err != nil {
		t.Fatal(err)
	}
	if got != "./data/vault.sqlite" {
		t.Fatalf("readDotEnvValue() = %q, want relative DB_PATH", got)
	}
}

func TestReadDotEnvValueIgnoresMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", ".env")
	if _, err := readDotEnvValue(path, "DB_PATH"); err != nil {
		t.Fatalf("missing .env error = %v, want nil", err)
	}
}
