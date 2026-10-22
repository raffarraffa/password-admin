package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	kdfIterations = 210_000
	keySize       = 32
	saltSize      = 16
)

var masterCheck = []byte("password-admin-vault-v1")

func deriveKey(password, salt []byte) []byte {
	mac := hmac.New(sha256.New, password)
	input := make([]byte, len(salt)+4)
	copy(input, salt)
	binary.BigEndian.PutUint32(input[len(salt):], 1)

	mac.Write(input)
	u := mac.Sum(nil)
	result := append([]byte(nil), u...)
	for i := 1; i < kdfIterations; i++ {
		mac.Reset()
		mac.Write(u)
		u = mac.Sum(nil)
		for j := range result {
			result[j] ^= u[j]
		}
	}
	return result[:keySize]
}

func encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crear cifrado: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crear cifrado autenticado: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generar nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crear descifrado: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crear descifrado autenticado: %w", err)
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("dato cifrado incompleto")
	}
	nonce, data := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, errors.New("no se pudo descifrar el dato")
	}
	return plaintext, nil
}

func validMasterKey(key, encryptedCheck []byte) bool {
	check, err := decrypt(key, encryptedCheck)
	return err == nil && subtle.ConstantTimeCompare(check, masterCheck) == 1
}
