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
	t.Setenv("BASE_PATH", "")
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, ".env"),
		[]byte("PORT=8124\nBASE_PATH=/pass-admin\n"), 0600); err != nil {
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
	basePath, err := configuredBasePath()
	if err != nil {
		t.Fatal(err)
	}
	if basePath != "/pass-admin" {
		t.Fatalf("configuredBasePath() = %q, want %q", basePath, "/pass-admin")
	}
}

func TestConfiguredPortRejectsInvalidValue(t *testing.T) {
	t.Setenv("PORT", "70000")
	if _, err := configuredPort(); err == nil {
		t.Fatal("configuredPort() accepted a port outside the valid range")
	}
}

func TestNormalizeBasePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
		fail  bool
	}{
		{input: "", want: ""},
		{input: "/", want: ""},
		{input: "pass-admin", want: "/pass-admin"},
		{input: "/pass-admin/", want: "/pass-admin"},
		{input: "/accounts/passwords", want: "/accounts/passwords"},
		{input: "/pass-admin/../other", fail: true},
		{input: "/pass-admin?x=1", fail: true},
		{input: "/pass-admin//other", fail: true},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := normalizeBasePath(test.input)
			if test.fail {
				if err == nil {
					t.Fatalf("normalizeBasePath(%q) = %q, want error", test.input, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("normalizeBasePath(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
