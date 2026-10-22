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
