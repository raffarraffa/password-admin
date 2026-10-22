package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Account struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	PasswordCount int    `json:"passwordCount"`
}

type PasswordEntry struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type PasswordSummary struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Username    string `json:"username"`
	AccountName string `json:"accountName"`
}

type Store struct {
	db  *sql.DB
	key []byte
}

func openStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("crear carpeta de datos: %w", err)
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolver ruta de la base de datos: %w", err)
	}
	store, err := openStoreDSN(absolutePath)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(absolutePath, 0600); err != nil {
		if closeErr := store.close(); closeErr != nil {
			return nil, fmt.Errorf("proteger archivo de base de datos: %w (cerrar conexión SQLite: %v)",
				err, closeErr)
		}
		return nil, fmt.Errorf("proteger archivo de base de datos: %w", err)
	}
	return store, nil
}

func openStoreDSN(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("abrir SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("conectar con SQLite: %w", err)
	}
	if err := configureStore(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

type sqlExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func configureStore(db sqlExecer) error {
	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value BLOB NOT NULL
		);
		CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE
		);
		CREATE TABLE IF NOT EXISTS passwords (
			id INTEGER PRIMARY KEY,
			account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			username BLOB NOT NULL,
			password BLOB NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("crear tablas SQLite: %w", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`PRAGMA foreign_keys = ON; PRAGMA secure_delete = ON; PRAGMA busy_timeout = 5000`); err != nil {
		return fmt.Errorf("configurar SQLite: %w", err)
	}
	return nil
}

func (s *Store) vaultSalt() ([]byte, error) {
	var salt []byte
	if err := s.db.QueryRow(`SELECT value FROM metadata WHERE key = 'salt'`).Scan(&salt); err != nil {
		return nil, fmt.Errorf("leer sal de la bóveda: %w", err)
	}
	if len(salt) != saltSize {
		return nil, errors.New("la sal de la bóveda tiene un formato inválido")
	}
	return salt, nil
}

func (s *Store) close() error {
	for i := range s.key {
		s.key[i] = 0
	}
	return s.db.Close()
}

func (s *Store) isInitialized() (bool, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM metadata WHERE key IN ('salt', 'check')`).Scan(&count); err != nil {
		return false, fmt.Errorf("consultar configuración de la bóveda: %w", err)
	}
	if count != 0 && count != 2 {
		return false, errors.New("la configuración de la bóveda está incompleta")
	}
	return count == 2, nil
}

func (s *Store) initialize(password string) error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM metadata`).Scan(&count); err != nil {
		return fmt.Errorf("consultar configuración de la bóveda: %w", err)
	}
	if count != 0 {
		return errors.New("la bóveda ya fue inicializada")
	}
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generar sal de cifrado: %w", err)
	}
	key := deriveKey([]byte(password), salt)
	check, err := encrypt(key, masterCheck)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("iniciar configuración de la bóveda: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO metadata(key, value) VALUES ('salt', ?), ('check', ?)`, salt, check); err != nil {
		return fmt.Errorf("guardar configuración de la bóveda: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("confirmar configuración de la bóveda: %w", err)
	}
	s.key = key
	return nil
}

func (s *Store) unlock(password string) (bool, error) {
	var salt, check []byte
	if err := s.db.QueryRow(`SELECT value FROM metadata WHERE key = 'salt'`).Scan(&salt); err != nil {
		return false, fmt.Errorf("leer sal de la bóveda: %w", err)
	}
	if err := s.db.QueryRow(`SELECT value FROM metadata WHERE key = 'check'`).Scan(&check); err != nil {
		return false, fmt.Errorf("leer verificación de la bóveda: %w", err)
	}
	key := deriveKey([]byte(password), salt)
	if !validMasterKey(key, check) {
		for i := range key {
			key[i] = 0
		}
		return false, nil
	}
	s.key = key
	return true, nil
}

func (s *Store) addAccount(name string) (Account, error) {
	result, err := s.db.Exec(`INSERT INTO accounts(name) VALUES (?)`, name)
	if err != nil {
		return Account{}, fmt.Errorf("crear cuenta (el nombre puede estar repetido): %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Account{}, fmt.Errorf("leer cuenta creada: %w", err)
	}
	return Account{ID: id, Name: name}, nil
}

func (s *Store) accountByID(id int64) (Account, error) {
	var account Account
	if err := s.db.QueryRow(`SELECT id, name FROM accounts WHERE id = ?`, id).Scan(&account.ID, &account.Name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Account{}, errors.New("la cuenta seleccionada no existe")
		}
		return Account{}, fmt.Errorf("consultar cuenta: %w", err)
	}
	return account, nil
}

func (s *Store) accounts() ([]Account, error) {
	rows, err := s.db.Query(`
		SELECT a.id, a.name, COUNT(p.id)
		FROM accounts a
		LEFT JOIN passwords p ON p.account_id = a.id
		GROUP BY a.id
		ORDER BY a.name COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("consultar cuentas: %w", err)
	}
	defer rows.Close()
	accounts := make([]Account, 0)
	for rows.Next() {
		var account Account
		if err := rows.Scan(&account.ID, &account.Name, &account.PasswordCount); err != nil {
			return nil, fmt.Errorf("leer cuenta: %w", err)
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recorrer cuentas: %w", err)
	}
	return accounts, nil
}

func (s *Store) addPasswords(accountID int64, entries []PasswordEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("iniciar importación: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO passwords(account_id, name, url, username, password) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparar importación: %w", err)
	}
	defer stmt.Close()
	for _, entry := range entries {
		username, err := encrypt(s.key, []byte(entry.Username))
		if err != nil {
			return err
		}
		password, err := encrypt(s.key, []byte(entry.Password))
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(accountID, entry.Name, entry.URL, username, password); err != nil {
			return fmt.Errorf("guardar contraseña de %q: %w", entry.URL, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("confirmar importación: %w", err)
	}
	return nil
}

func (s *Store) importPasswords(accountID int64, newAccountName string, entries []PasswordEntry) (Account, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Account{}, fmt.Errorf("iniciar importación: %w", err)
	}
	defer tx.Rollback()

	var account Account
	if accountID == 0 {
		if newAccountName == "" {
			return Account{}, errors.New("selecciona una cuenta o escribe el nombre de una nueva")
		}
		result, err := tx.Exec(`INSERT INTO accounts(name) VALUES (?)`, newAccountName)
		if err != nil {
			return Account{}, fmt.Errorf("crear cuenta (el nombre puede estar repetido): %w", err)
		}
		account.ID, err = result.LastInsertId()
		if err != nil {
			return Account{}, fmt.Errorf("leer cuenta creada: %w", err)
		}
		account.Name = newAccountName
	} else {
		if newAccountName != "" {
			return Account{}, errors.New("elige una cuenta existente o crea una nueva, no ambas")
		}
		if err := tx.QueryRow(`SELECT id, name FROM accounts WHERE id = ?`, accountID).Scan(&account.ID, &account.Name); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Account{}, errors.New("la cuenta seleccionada no existe")
			}
			return Account{}, fmt.Errorf("consultar cuenta: %w", err)
		}
	}

	stmt, err := tx.Prepare(`INSERT INTO passwords(account_id, name, url, username, password) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return Account{}, fmt.Errorf("preparar importación: %w", err)
	}
	defer stmt.Close()
	for _, entry := range entries {
		username, err := encrypt(s.key, []byte(entry.Username))
		if err != nil {
			return Account{}, err
		}
		password, err := encrypt(s.key, []byte(entry.Password))
		if err != nil {
			return Account{}, err
		}
		if _, err := stmt.Exec(account.ID, entry.Name, entry.URL, username, password); err != nil {
			return Account{}, fmt.Errorf("guardar contraseña de %q: %w", entry.URL, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Account{}, fmt.Errorf("confirmar importación: %w", err)
	}
	return account, nil
}

func (s *Store) passwords(accountID int64) ([]PasswordEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, name, url, username, password FROM passwords
		WHERE (? = 0 OR account_id = ?)
		ORDER BY url COLLATE NOCASE, id
	`, accountID, accountID)
	if err != nil {
		return nil, fmt.Errorf("consultar contraseñas: %w", err)
	}
	defer rows.Close()
	entries := make([]PasswordEntry, 0)
	for rows.Next() {
		var entry PasswordEntry
		var username, password []byte
		if err := rows.Scan(&entry.ID, &entry.Name, &entry.URL, &username, &password); err != nil {
			return nil, fmt.Errorf("leer contraseña: %w", err)
		}
		plainUsername, err := decrypt(s.key, username)
		if err != nil {
			return nil, fmt.Errorf("descifrar usuario para %q: %w", entry.URL, err)
		}
		plainPassword, err := decrypt(s.key, password)
		if err != nil {
			return nil, fmt.Errorf("descifrar contraseña para %q: %w", entry.URL, err)
		}
		entry.Username, entry.Password = string(plainUsername), string(plainPassword)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recorrer contraseñas: %w", err)
	}
	return entries, nil
}

func (s *Store) passwordSummaries(accountID int64) ([]PasswordSummary, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.name, p.url, p.username, a.name
		FROM passwords p
		JOIN accounts a ON a.id = p.account_id
		WHERE (? = 0 OR p.account_id = ?)
		ORDER BY p.url COLLATE NOCASE, p.id
	`, accountID, accountID)
	if err != nil {
		return nil, fmt.Errorf("consultar contraseñas: %w", err)
	}
	defer rows.Close()
	summaries := make([]PasswordSummary, 0)
	for rows.Next() {
		var summary PasswordSummary
		var username []byte
		if err := rows.Scan(&summary.ID, &summary.Name, &summary.URL, &username, &summary.AccountName); err != nil {
			return nil, fmt.Errorf("leer resumen de contraseña: %w", err)
		}
		plainUsername, err := decrypt(s.key, username)
		if err != nil {
			return nil, fmt.Errorf("descifrar usuario para %q: %w", summary.URL, err)
		}
		summary.Username = string(plainUsername)
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recorrer contraseñas: %w", err)
	}
	return summaries, nil
}

func (s *Store) passwordByID(id int64) (PasswordEntry, error) {
	var entry PasswordEntry
	var username, password []byte
	if err := s.db.QueryRow(`SELECT id, name, url, username, password FROM passwords WHERE id = ?`, id).
		Scan(&entry.ID, &entry.Name, &entry.URL, &username, &password); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PasswordEntry{}, errors.New("la contraseña seleccionada no existe")
		}
		return PasswordEntry{}, fmt.Errorf("consultar contraseña: %w", err)
	}
	plainUsername, err := decrypt(s.key, username)
	if err != nil {
		return PasswordEntry{}, fmt.Errorf("descifrar usuario para %q: %w", entry.URL, err)
	}
	plainPassword, err := decrypt(s.key, password)
	if err != nil {
		return PasswordEntry{}, fmt.Errorf("descifrar contraseña para %q: %w", entry.URL, err)
	}
	entry.Username, entry.Password = string(plainUsername), string(plainPassword)
	return entry, nil
}

func (s *Store) deletePassword(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM passwords WHERE id = ?`, id); err != nil {
		return fmt.Errorf("eliminar contraseña: %w", err)
	}
	return nil
}
