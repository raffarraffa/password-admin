package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestMemoryStoreSerializeDeserialize(t *testing.T) {
	store, err := openMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()

	if err := store.initialize("correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	account, err := store.addAccount("Personal")
	if err != nil {
		t.Fatal(err)
	}
	entry := PasswordEntry{
		Name:     "Example",
		URL:      "https://example.test",
		Username: "alice",
		Password: "secret-value",
	}
	if err := store.addPasswords(account.ID, []PasswordEntry{entry}); err != nil {
		t.Fatal(err)
	}

	database, err := store.serialize()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(database)
	if bytes.Contains(database, []byte(entry.Password)) {
		t.Fatal("serialized vault unexpectedly contains a plaintext password")
	}

	restored, err := openUninitializedMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer restored.close()
	if err := restored.deserialize(database); err != nil {
		t.Fatalf("deserialize() error = %v", err)
	}
	unlocked, err := restored.unlock("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !unlocked {
		t.Fatal("restored store did not unlock with the correct password")
	}
	accounts, err := restored.accounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].Name != "Personal" {
		t.Fatalf("restored accounts = %#v, want one Personal account", accounts)
	}
	passwords, err := restored.passwords(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(passwords) != 1 || passwords[0].Password != entry.Password ||
		passwords[0].Username != entry.Username {
		t.Fatalf("restored passwords = %#v, want %#v", passwords, entry)
	}
}

func TestReadOnlyVaultSnapshotImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "folder with spaces", "vault.sqlite")
	source, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := source.initialize("correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	account, err := source.addAccount("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := source.addPasswords(account.ID, []PasswordEntry{{
		Name: "Example", URL: "https://example.test", Username: "alice", Password: "secret",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := source.close(); err != nil {
		t.Fatal(err)
	}

	local, err := openReadOnlyStore(path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := local.serialize()
	if err != nil {
		t.Fatal(err)
	}
	if err := local.close(); err != nil {
		t.Fatal(err)
	}
	defer clear(snapshot)

	imported, err := openUninitializedMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	defer imported.close()
	if err := imported.deserialize(snapshot); err != nil {
		t.Fatal(err)
	}
	unlocked, err := imported.unlock("correct horse battery staple")
	if err != nil || !unlocked {
		t.Fatalf("unlock imported vault = (%v, %v), want (true, nil)", unlocked, err)
	}
	entries, err := imported.passwords(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Password != "secret" {
		t.Fatalf("imported entries = %#v, want the original password", entries)
	}
}
