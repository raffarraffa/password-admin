package main

import (
	"bytes"
	"testing"
)

func TestPortableTokenRoundTrip(t *testing.T) {
	config := GitHubConfig{Owner: "example-user", Repo: "private-vault"}
	token := "github_pat_secret-value"
	encoded, err := encodePortableToken(config, "master password", token)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(encoded)
	if bytes.Contains(encoded, []byte(token)) {
		t.Fatal("portable credential file contains the plaintext token")
	}
	got, err := decodePortableToken(config, "master password", encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got != token {
		t.Fatalf("decoded token = %q, want %q", got, token)
	}
}

func TestPortableTokenRejectsWrongPasswordAndRepository(t *testing.T) {
	config := GitHubConfig{Owner: "example-user", Repo: "private-vault"}
	encoded, err := encodePortableToken(config, "master password", "github_pat_secret-value")
	if err != nil {
		t.Fatal(err)
	}
	defer clear(encoded)
	if _, err := decodePortableToken(config, "wrong password", encoded); err == nil {
		t.Fatal("portable credentials accepted a wrong password")
	}
	if _, err := decodePortableToken(GitHubConfig{Owner: "other-user", Repo: "other-vault"}, "master password", encoded); err == nil {
		t.Fatal("portable credentials accepted a different repository")
	}
}
