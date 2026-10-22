package main

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := deriveKey([]byte("correct horse battery staple"), []byte("0123456789abcdef"))
	plaintext := []byte("secret text")
	ciphertext, err := encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatal("ciphertext contains plaintext")
	}
	got, err := decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("decrypt() error = %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("decrypt() = %q, want %q", got, plaintext)
	}
}

func TestDecryptRejectsTampering(t *testing.T) {
	key := deriveKey([]byte("correct horse battery staple"), []byte("0123456789abcdef"))
	ciphertext, err := encrypt(key, []byte("secret text"))
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err := decrypt(key, ciphertext); err == nil {
		t.Fatal("decrypt() accepted modified ciphertext")
	}
}

func TestVaultArchiveRoundTrip(t *testing.T) {
	salt := []byte("0123456789abcdef")
	key := deriveKey([]byte("correct horse battery staple"), salt)
	database := []byte("SQLite database bytes")
	envelope, err := encryptVaultArchive(key, salt, database)
	if err != nil {
		t.Fatalf("encryptVaultArchive() error = %v", err)
	}
	if bytes.Contains(envelope, database) {
		t.Fatal("vault envelope contains the plaintext database")
	}
	got, gotSalt, err := decryptVaultArchive("correct horse battery staple", envelope)
	if err != nil {
		t.Fatalf("decryptVaultArchive() error = %v", err)
	}
	if !bytes.Equal(got, database) || !bytes.Equal(gotSalt, salt) {
		t.Fatalf("decryptVaultArchive() = (%q, %q), want (%q, %q)", got, gotSalt, database, salt)
	}
}

func TestVaultArchiveRejectsWrongPasswordAndTampering(t *testing.T) {
	salt := []byte("0123456789abcdef")
	key := deriveKey([]byte("correct horse battery staple"), salt)
	envelope, err := encryptVaultArchive(key, salt, []byte("database"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := decryptVaultArchive("wrong password", envelope); err == nil {
		t.Fatal("decryptVaultArchive() accepted a wrong password")
	}
	envelope[len(envelope)-1] ^= 1
	if _, _, err := decryptVaultArchive("correct horse battery staple", envelope); err == nil {
		t.Fatal("decryptVaultArchive() accepted a modified archive")
	}
}
