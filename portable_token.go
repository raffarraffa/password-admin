package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var portableTokenMagic = []byte("PADMTOKEN1")

type portableCredential struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Token string `json:"token"`
}

func portableTokenPath(config GitHubConfig) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("localizar la carpeta de la aplicación: %w", err)
	}
	repositoryID := sha256.Sum256([]byte(strings.ToLower(config.Owner + "/" + config.Repo)))
	name := "password-admin.credentials-" + hex.EncodeToString(repositoryID[:8])
	return filepath.Join(filepath.Dir(executable), name), nil
}

func savePortableToken(config GitHubConfig, password, token string) error {
	path, err := portableTokenPath(config)
	if err != nil {
		return err
	}
	data, err := encodePortableToken(config, password, token)
	if err != nil {
		return err
	}
	defer clear(data)

	temp, err := os.CreateTemp(filepath.Dir(path), ".password-admin-credentials-*")
	if err != nil {
		return fmt.Errorf("crear archivo temporal de credenciales: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("proteger archivo de credenciales: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("escribir archivo de credenciales: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("guardar archivo de credenciales: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("cerrar archivo de credenciales: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		if _, statErr := os.Stat(path); statErr != nil {
			return fmt.Errorf("instalar archivo de credenciales portátil: %w", err)
		}
		if removeErr := os.Remove(path); removeErr != nil {
			return fmt.Errorf("reemplazar archivo de credenciales portátil: %w", removeErr)
		}
		if renameErr := os.Rename(tempPath, path); renameErr != nil {
			return fmt.Errorf("instalar archivo de credenciales portátil: %w", renameErr)
		}
	}
	return nil
}

func encodePortableToken(config GitHubConfig, password, token string) ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generar sal para proteger el token: %w", err)
	}
	plaintext, err := json.Marshal(portableCredential{
		Owner: config.Owner,
		Repo:  config.Repo,
		Token: token,
	})
	if err != nil {
		return nil, fmt.Errorf("preparar credenciales portátiles: %w", err)
	}
	key := deriveKey([]byte(password), salt)
	defer clear(key)
	ciphertext, err := encrypt(key, plaintext)
	clear(plaintext)
	if err != nil {
		return nil, fmt.Errorf("cifrar token de GitHub: %w", err)
	}
	data := make([]byte, 0, len(portableTokenMagic)+len(salt)+len(ciphertext))
	data = append(data, portableTokenMagic...)
	data = append(data, salt...)
	data = append(data, ciphertext...)
	return data, nil
}

func loadPortableToken(config GitHubConfig, password string) (string, error) {
	path, err := portableTokenPath(config)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	defer clear(data)
	return decodePortableToken(config, password, data)
}

func decodePortableToken(config GitHubConfig, password string, data []byte) (string, error) {
	minimumLength := len(portableTokenMagic) + saltSize + 12 + 16
	if len(data) < minimumLength || !bytes.Equal(data[:len(portableTokenMagic)], portableTokenMagic) {
		return "", errors.New("el archivo portátil de credenciales tiene un formato inválido")
	}
	saltStart := len(portableTokenMagic)
	saltEnd := saltStart + saltSize
	key := deriveKey([]byte(password), data[saltStart:saltEnd])
	defer clear(key)
	plaintext, err := decrypt(key, data[saltEnd:])
	if err != nil {
		return "", errors.New("contraseña maestra incorrecta o archivo portátil de credenciales dañado")
	}
	defer clear(plaintext)
	var credential portableCredential
	if err := json.Unmarshal(plaintext, &credential); err != nil {
		return "", fmt.Errorf("interpretar archivo portátil de credenciales: %w", err)
	}
	if credential.Owner != config.Owner || credential.Repo != config.Repo {
		return "", errors.New("el archivo portátil de credenciales pertenece a otro repositorio")
	}
	if credential.Token == "" {
		return "", errors.New("el archivo portátil no contiene un token de GitHub")
	}
	return credential.Token, nil
}
