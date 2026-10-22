package main

import (
	"bytes"
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
var vaultEnvelopeMagic = []byte("PADMVAULT2")

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

func encryptVaultArchive(key, salt, database []byte) ([]byte, error) {
	if len(salt) != saltSize {
		return nil, errors.New("sal de bóveda no válida")
	}
	ciphertext, err := encrypt(key, database)
	if err != nil {
		return nil, fmt.Errorf("cifrar el contenido completo de la bóveda: %w", err)
	}
	envelope := make([]byte, 0, len(vaultEnvelopeMagic)+len(salt)+len(ciphertext))
	envelope = append(envelope, vaultEnvelopeMagic...)
	envelope = append(envelope, salt...)
	envelope = append(envelope, ciphertext...)
	return envelope, nil
}

func decryptVaultArchive(password string, envelope []byte) ([]byte, []byte, error) {
	minimumLength := len(vaultEnvelopeMagic) + saltSize + 12 + 16
	if len(envelope) < minimumLength || !bytes.Equal(envelope[:len(vaultEnvelopeMagic)], vaultEnvelopeMagic) {
		return nil, nil, errors.New("el archivo remoto no es una bóveda Password Admin válida")
	}
	saltStart := len(vaultEnvelopeMagic)
	saltEnd := saltStart + saltSize
	key := deriveKey([]byte(password), envelope[saltStart:saltEnd])
	defer clear(key)
	database, err := decrypt(key, envelope[saltEnd:])
	if err != nil {
		return nil, nil, errors.New("contraseña maestra incorrecta o archivo remoto dañado")
	}
	return database, append([]byte(nil), envelope[saltStart:saltEnd]...), nil
}
