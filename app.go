package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"unicode/utf8"
)

type VaultStatus struct {
	Initialized bool `json:"initialized"`
	Unlocked    bool `json:"unlocked"`
}

type ImportResult struct {
	AccountName string `json:"accountName"`
	Count       int    `json:"count"`
}

type App struct {
	mu    sync.Mutex
	store *Store
}

func NewApp(databasePath string) (*App, error) {
	store, err := openStore(databasePath)
	if err != nil {
		return nil, err
	}
	return &App{store: store}, nil
}

func (a *App) shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closeStoreLocked()
}

func (a *App) closeStoreLocked() {
	if a.store == nil {
		return
	}
	if err := a.store.close(); err != nil {
		fmt.Fprintf(os.Stderr, "cerrar la base SQLite: %v\n", err)
	}
	a.store = nil
}

func (a *App) VaultStatus() (VaultStatus, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.store == nil {
		return VaultStatus{}, errors.New("la base de datos local no está disponible")
	}
	initialized, err := a.store.isInitialized()
	if err != nil {
		return VaultStatus{}, err
	}
	return VaultStatus{Initialized: initialized, Unlocked: len(a.store.key) > 0}, nil
}

func (a *App) SetupVault(password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if utf8.RuneCountInString(password) < 8 {
		return errors.New("la contraseña maestra debe tener al menos 8 caracteres")
	}
	if a.store == nil {
		return errors.New("la base de datos local no está disponible")
	}
	initialized, err := a.store.isInitialized()
	if err != nil {
		return err
	}
	if initialized {
		return errors.New("la bóveda ya fue inicializada; desbloquéala con su contraseña maestra")
	}
	return a.store.initialize(password)
}

func (a *App) UnlockVault(password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.store == nil {
		return errors.New("la base de datos local no está disponible")
	}
	initialized, err := a.store.isInitialized()
	if err != nil {
		return err
	}
	if !initialized {
		return errors.New("todavía no hay una bóveda; crea una contraseña maestra")
	}
	unlocked, err := a.store.unlock(password)
	if err != nil {
		return err
	}
	if !unlocked {
		return errors.New("contraseña maestra incorrecta o base de datos dañada")
	}
	return nil
}

func (a *App) Accounts() ([]Account, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.requireUnlockedLocked(); err != nil {
		return nil, err
	}
	return a.store.accounts()
}

func (a *App) CreateAccount(name string) (Account, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.requireUnlockedLocked(); err != nil {
		return Account{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Account{}, errors.New("el nombre de la cuenta no puede estar vacío")
	}
	return a.store.addAccount(name)
}

func (a *App) Passwords(accountID int64) ([]PasswordSummary, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.requireUnlockedLocked(); err != nil {
		return nil, err
	}
	return a.store.passwordSummaries(accountID)
}

func (a *App) PasswordDetails(entryID int64) (PasswordEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.requireUnlockedLocked(); err != nil {
		return PasswordEntry{}, err
	}
	return a.store.passwordByID(entryID)
}

func (a *App) PreviewCSV(content string) (int, error) {
	entries, err := readChromeCSV(strings.NewReader(content))
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}

func (a *App) ImportCSV(content string, accountID int64, newAccountName string) (ImportResult, error) {
	entries, err := readChromeCSV(strings.NewReader(content))
	if err != nil {
		return ImportResult{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.requireUnlockedLocked(); err != nil {
		return ImportResult{}, err
	}
	account, err := a.store.importPasswords(accountID, strings.TrimSpace(newAccountName), entries)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{AccountName: account.Name, Count: len(entries)}, nil
}

func (a *App) ExportCSV(accountID int64) ([]byte, error) {
	a.mu.Lock()
	if err := a.requireUnlockedLocked(); err != nil {
		a.mu.Unlock()
		return nil, err
	}
	if accountID != 0 {
		if _, err := a.store.accountByID(accountID); err != nil {
			a.mu.Unlock()
			return nil, err
		}
	}
	entries, err := a.store.passwords(accountID)
	a.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		if accountID == 0 {
			return nil, errors.New("no hay contraseñas para exportar")
		}
		return nil, errors.New("la cuenta seleccionada no tiene contraseñas para exportar")
	}
	var output bytes.Buffer
	if err := writeChromeCSV(&output, entries); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func (a *App) DeletePassword(id int64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.requireUnlockedLocked(); err != nil {
		return err
	}
	return a.store.deletePassword(id)
}

func (a *App) requireUnlockedLocked() error {
	if a.store == nil {
		return errors.New("la base de datos local no está disponible")
	}
	if len(a.store.key) == 0 {
		return errors.New("la bóveda está bloqueada")
	}
	return nil
}
