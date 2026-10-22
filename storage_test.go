package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreEncryptsAndReadsAccountPasswords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.sqlite")
	store, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore() error = %v", err)
	}
	defer store.close()

	initialized, err := store.isInitialized()
	if err != nil {
		t.Fatalf("isInitialized() error = %v", err)
	}
	if initialized {
		t.Fatal("new store is already initialized")
	}
	if err := store.initialize("a sufficiently long master password"); err != nil {
		t.Fatalf("initialize() error = %v", err)
	}
	if ok, err := store.unlock("wrong master password"); err != nil || ok {
		t.Fatalf("unlock() with wrong password = (%v, %v), want (false, nil)", ok, err)
	}
	if ok, err := store.unlock("a sufficiently long master password"); err != nil || !ok {
		t.Fatalf("unlock() = (%v, %v), want (true, nil)", ok, err)
	}
	account, err := store.addAccount("Personal")
	if err != nil {
		t.Fatalf("addAccount() error = %v", err)
	}
	want := []PasswordEntry{{
		Name: "Example", URL: "https://example.com",
		Username: "user@example.com", Password: "a secret password",
	}}
	if err := store.addPasswords(account.ID, want); err != nil {
		t.Fatalf("addPasswords() error = %v", err)
	}
	got, err := store.passwords(account.ID)
	if err != nil {
		t.Fatalf("passwords() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("passwords() returned %d entries, want 1", len(got))
	}
	want[0].ID = got[0].ID
	if got[0] != want[0] {
		t.Fatalf("passwords() = %#v, want %#v", got, want)
	}
}

func TestStoreDoesNotPersistPlaintextPasswords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.sqlite")
	store, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore() error = %v", err)
	}
	if err := store.initialize("a sufficiently long master password"); err != nil {
		store.close()
		t.Fatalf("initialize() error = %v", err)
	}
	account, err := store.addAccount("Personal")
	if err != nil {
		store.close()
		t.Fatalf("addAccount() error = %v", err)
	}
	const secret = "distinctive-secret-value"
	if err := store.addPasswords(account.ID, []PasswordEntry{{
		Name: "Example", URL: "https://example.com", Username: "private-user", Password: secret,
	}}); err != nil {
		store.close()
		t.Fatalf("addPasswords() error = %v", err)
	}
	if err := store.close(); err != nil {
		t.Fatalf("close() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if bytes.Contains(data, []byte(secret)) || bytes.Contains(data, []byte("private-user")) {
		t.Fatal("SQLite file contains plaintext credentials")
	}
}

func TestImportPasswordsCreatesAccountAtomically(t *testing.T) {
	store, err := openStore(filepath.Join(t.TempDir(), "vault.sqlite"))
	if err != nil {
		t.Fatalf("openStore() error = %v", err)
	}
	defer store.close()
	if err := store.initialize("a sufficiently long master password"); err != nil {
		t.Fatalf("initialize() error = %v", err)
	}

	entries := []PasswordEntry{{
		Name: "Example", URL: "https://example.com", Username: "user", Password: "secret",
	}}
	account, err := store.importPasswords(0, "Work", entries)
	if err != nil {
		t.Fatalf("importPasswords() error = %v", err)
	}
	summaries, err := store.passwordSummaries(account.ID)
	if err != nil {
		t.Fatalf("passwordSummaries() error = %v", err)
	}
	if len(summaries) != 1 || summaries[0].Username != "user" {
		t.Fatalf("passwordSummaries() = %#v", summaries)
	}
	if _, err := store.importPasswords(0, "Work", entries); err == nil {
		t.Fatal("importPasswords() accepted duplicate account name")
	}
	accounts, err := store.accounts()
	if err != nil {
		t.Fatalf("accounts() error = %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("failed import left an extra account: %#v", accounts)
	}
	if accounts[0].PasswordCount != 1 {
		t.Fatalf("account password count = %d, want 1", accounts[0].PasswordCount)
	}
	details, err := store.passwordByID(summaries[0].ID)
	if err != nil {
		t.Fatalf("passwordByID() error = %v", err)
	}
	if details.Password != "secret" {
		t.Fatal("passwordByID() returned incorrect password")
	}

	personal, err := store.addAccount("Personal")
	if err != nil {
		t.Fatalf("addAccount() error = %v", err)
	}
	if err := store.addPasswords(personal.ID, []PasswordEntry{{
		Name: "Other", URL: "https://other.example", Username: "another-user", Password: "another-secret",
	}}); err != nil {
		t.Fatalf("addPasswords() error = %v", err)
	}
	allSummaries, err := store.passwordSummaries(0)
	if err != nil {
		t.Fatalf("passwordSummaries(all) error = %v", err)
	}
	if len(allSummaries) != 2 {
		t.Fatalf("passwordSummaries(all) returned %d entries, want 2", len(allSummaries))
	}
	if allSummaries[0].AccountName == allSummaries[1].AccountName {
		t.Fatalf("all-account summaries do not identify their accounts: %#v", allSummaries)
	}
	allPasswords, err := store.passwords(0)
	if err != nil {
		t.Fatalf("passwords(all) error = %v", err)
	}
	if len(allPasswords) != 2 {
		t.Fatalf("passwords(all) returned %d entries, want 2", len(allPasswords))
	}
	workPasswords, err := store.passwords(account.ID)
	if err != nil {
		t.Fatalf("passwords(account) error = %v", err)
	}
	if len(workPasswords) != 1 || workPasswords[0].Password != "secret" {
		t.Fatalf("passwords(account) = %#v, want only the Work entry", workPasswords)
	}
}
