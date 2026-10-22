package main

import (
	"path/filepath"
	"testing"
)

func TestPersistentVaultSurvivesRestart(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "data", "password-admin.sqlite")
	app, err := NewApp(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	status, err := app.VaultStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Initialized || status.Unlocked {
		t.Fatalf("initial status = %#v, want an uninitialized locked vault", status)
	}
	if err := app.SetupVault("correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	account, err := app.CreateAccount("Server")
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.ImportCSV(
		"name,url,username,password\nExample,https://example.test,alice,secret-value\n",
		account.ID,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 {
		t.Fatalf("ImportCSV count = %d, want 1", result.Count)
	}
	app.shutdown()

	restarted, err := NewApp(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.shutdown()
	status, err = restarted.VaultStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Initialized || status.Unlocked {
		t.Fatalf("restarted status = %#v, want an initialized locked vault", status)
	}
	if err := restarted.UnlockVault("wrong password"); err == nil {
		t.Fatal("UnlockVault accepted the wrong password")
	}
	if err := restarted.UnlockVault("correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	accounts, err := restarted.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].Name != "Server" || accounts[0].PasswordCount != 1 {
		t.Fatalf("persisted accounts = %#v, want Server with one password", accounts)
	}
	passwords, err := restarted.PasswordDetails(1)
	if err != nil {
		t.Fatal(err)
	}
	if passwords.Password != "secret-value" {
		t.Fatalf("persisted password = %q, want secret-value", passwords.Password)
	}
}
