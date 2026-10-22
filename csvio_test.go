package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadChromeCSV(t *testing.T) {
	input := "\ufeffname,url,username,password\r\nExample,https://example.com,user@example.com,\"p,ass\nword\"\r\n"
	entries, err := readChromeCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("readChromeCSV() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("readChromeCSV() returned %d entries, want 1", len(entries))
	}
	got := entries[0]
	if got.Name != "Example" || got.URL != "https://example.com" ||
		got.Username != "user@example.com" || got.Password != "p,ass\nword" {
		t.Fatalf("readChromeCSV() entry = %#v", got)
	}
}

func TestChromeCSVRoundTrip(t *testing.T) {
	want := []PasswordEntry{{
		Name: "Example", URL: "https://example.com", Username: "user",
		Password: "comma, quote \" and newline\n",
	}}
	var output bytes.Buffer
	if err := writeChromeCSV(&output, want); err != nil {
		t.Fatalf("writeChromeCSV() error = %v", err)
	}
	got, err := readChromeCSV(&output)
	if err != nil {
		t.Fatalf("readChromeCSV() error = %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func TestReadChromeCSVRequiresChromeColumns(t *testing.T) {
	_, err := readChromeCSV(strings.NewReader("site,user,secret\nhttps://example.com,user,pass\n"))
	if err == nil {
		t.Fatal("readChromeCSV() succeeded without required columns")
	}
}
