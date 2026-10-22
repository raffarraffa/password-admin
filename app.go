package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	mu          sync.Mutex
	store       *Store
	ctx         context.Context
	development bool
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.development = runtime.Environment(ctx).BuildType == "dev"
}

func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.store != nil {
		if err := a.store.close(); err != nil {
			fmt.Fprintf(os.Stderr, "cerrar la base SQLite: %v\n", err)
		}
		a.store = nil
	}
}

func (a *App) ensureStore() error {
	if a.store != nil {
		return nil
	}
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("localizar el ejecutable: %w", err)
	}
	workingDir := ""
	if a.development {
		workingDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("localizar la carpeta del proyecto: %w", err)
		}
	}
	databasePath, err := resolveVaultPath(executablePath, workingDir, a.development)
	if err != nil {
		return err
	}
	store, err := openStore(databasePath)
	if err != nil {
		return err
	}
	a.store = store
	return nil
}

func resolveVaultPath(executablePath, workingDir string, development bool) (string, error) {
	executableDir := filepath.Dir(executablePath)
	if !development {
		return filepath.Join(executableDir, "vault.sqlite"), nil
	}

	projectPath := filepath.Join(workingDir, "vault.sqlite")
	legacyPath := filepath.Join(executableDir, "vault.sqlite")
	if _, err := os.Stat(projectPath); err == nil {
		return projectPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("comprobar la base SQLite del proyecto: %w", err)
	}
	if _, err := os.Stat(legacyPath); err == nil {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			source := legacyPath + suffix
			if _, err := os.Stat(source); err == nil {
				if err := os.Rename(source, projectPath+suffix); err != nil {
					return "", fmt.Errorf("mover la base SQLite de desarrollo a la raíz del proyecto: %w", err)
				}
			} else if !os.IsNotExist(err) {
				return "", fmt.Errorf("comprobar archivo SQLite de desarrollo: %w", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("comprobar la base SQLite anterior: %w", err)
	}
	return projectPath, nil
}

func (a *App) VaultStatus() (VaultStatus, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.ensureStore(); err != nil {
		return VaultStatus{}, err
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
	if err := a.ensureStore(); err != nil {
		return err
	}
	return a.store.initialize(password)
}

func (a *App) UnlockVault(password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.ensureStore(); err != nil {
		return err
	}
	unlocked, err := a.store.unlock(password)
	if err != nil {
		return err
	}
	if !unlocked {
		return errors.New("contraseña maestra incorrecta")
	}
	return nil
}

func (a *App) Accounts() ([]Account, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.requireUnlocked(); err != nil {
		return nil, err
	}
	return a.store.accounts()
}

func (a *App) CreateAccount(name string) (Account, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.requireUnlocked(); err != nil {
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
	if err := a.requireUnlocked(); err != nil {
		return nil, err
	}
	return a.store.passwordSummaries(accountID)
}

func (a *App) PasswordDetails(entryID int64) (PasswordEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.requireUnlocked(); err != nil {
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
	if err := a.requireUnlocked(); err != nil {
		return ImportResult{}, err
	}
	account, err := a.store.importPasswords(accountID, strings.TrimSpace(newAccountName), entries)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{AccountName: account.Name, Count: len(entries)}, nil
}

func (a *App) ExportCSV(accountID int64) (bool, error) {
	a.mu.Lock()
	if err := a.requireUnlocked(); err != nil {
		a.mu.Unlock()
		return false, err
	}
	accountName := "todas las cuentas"
	if accountID != 0 {
		account, err := a.store.accountByID(accountID)
		if err != nil {
			a.mu.Unlock()
			return false, err
		}
		accountName = account.Name
	}
	entries, err := a.store.passwords(accountID)
	a.mu.Unlock()
	if err != nil {
		return false, err
	}
	if len(entries) == 0 {
		if accountID == 0 {
			return false, errors.New("no hay contraseñas para exportar")
		}
		return false, errors.New("la cuenta seleccionada no tiene contraseñas para exportar")
	}
	var output bytes.Buffer
	if err := writeChromeCSV(&output, entries); err != nil {
		return false, err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "chrome-passwords.csv",
		Title:           "Exportar contraseñas de " + accountName,
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV", Pattern: "*.csv"},
		},
	})
	if err != nil {
		return false, fmt.Errorf("elegir dónde guardar el CSV: %w", err)
	}
	if path == "" {
		return false, nil
	}
	if err := os.WriteFile(path, output.Bytes(), 0600); err != nil {
		return false, fmt.Errorf("guardar CSV: %w", err)
	}
	return true, nil
}

func (a *App) DeletePassword(id int64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.requireUnlocked(); err != nil {
		return err
	}
	return a.store.deletePassword(id)
}

func (a *App) requireUnlocked() error {
	if err := a.ensureStore(); err != nil {
		return err
	}
	if len(a.store.key) == 0 {
		return errors.New("la bóveda está bloqueada")
	}
	return nil
}
