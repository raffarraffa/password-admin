package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type VaultStatus struct {
	Configured   bool         `json:"configured"`
	HasToken     bool         `json:"hasToken"`
	Initialized  bool         `json:"initialized"`
	Unlocked     bool         `json:"unlocked"`
	RemoteExists bool         `json:"remoteExists"`
	Config       GitHubConfig `json:"config"`
}

type ImportResult struct {
	AccountName string `json:"accountName"`
	Count       int    `json:"count"`
}

type App struct {
	mu             sync.Mutex
	store          *Store
	ctx            context.Context
	remote         *githubVault
	remoteEnvelope []byte
	remoteExists   bool
	dirty          bool
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.mu.Lock()
	if !a.dirty {
		a.mu.Unlock()
		return false
	}
	err := a.syncLocked()
	a.mu.Unlock()
	if err == nil {
		return false
	}
	runtime.EventsEmit(ctx, "vault-sync-error", err.Error())
	return true
}

func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.dirty {
		if err := a.syncLocked(); err != nil {
			fmt.Fprintf(os.Stderr, "no se pudo guardar la bóveda remota antes de cerrar: %v\n", err)
		}
	}
	a.closeStoreLocked()
	clear(a.remoteEnvelope)
	a.remoteEnvelope = nil
	a.remote = nil
}

func (a *App) closeStoreLocked() {
	if a.store == nil {
		return
	}
	if err := a.store.close(); err != nil {
		fmt.Fprintf(os.Stderr, "cerrar la base SQLite en memoria: %v\n", err)
	}
	a.store = nil
}

func (a *App) VaultStatus() (VaultStatus, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	config, err := loadGitHubConfig()
	if err != nil {
		return VaultStatus{}, err
	}
	if config.Owner == "" || config.Repo == "" {
		return VaultStatus{Config: config}, nil
	}
	if a.remote == nil {
		tokenPath, pathErr := portableTokenPath(config)
		if pathErr != nil {
			return VaultStatus{}, pathErr
		}
		if _, statErr := os.Stat(tokenPath); statErr == nil {
			return VaultStatus{
				Configured: true, HasToken: true,
				Initialized: config.Initialized, Config: config,
			}, nil
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return VaultStatus{}, fmt.Errorf("comprobar archivo portátil de credenciales: %w", statErr)
		}
		token, tokenErr := readGitHubToken(config)
		if tokenErr != nil {
			return VaultStatus{Configured: true, Config: config}, tokenErr
		}
		if token == "" {
			return VaultStatus{Configured: true, Config: config}, nil
		}
		if err := a.connectGitHubLocked(config, token); err != nil {
			return VaultStatus{}, err
		}
		config.Initialized = a.remoteExists
		if err := saveGitHubConfig(config); err != nil {
			return VaultStatus{}, err
		}
	}
	initialized := a.remoteExists
	if a.store != nil {
		initialized, err = a.store.isInitialized()
		if err != nil {
			return VaultStatus{}, err
		}
		initialized = initialized || a.remoteExists
	}
	return VaultStatus{
		Configured:   true,
		HasToken:     true,
		Initialized:  initialized,
		Unlocked:     a.store != nil && len(a.store.key) > 0,
		RemoteExists: a.remoteExists,
		Config:       config,
	}, nil
}

func (a *App) ConfigureGitHub(owner, repo, token string) (VaultStatus, error) {
	config, err := validateGitHubSettings(owner, repo, token)
	if err != nil {
		return VaultStatus{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.dirty {
		return VaultStatus{}, errors.New("hay cambios pendientes en la bóveda anterior; sincronízalos antes de cambiar de repositorio")
	}

	remote := newGitHubVault(config, strings.TrimSpace(token))
	envelope, exists, err := remote.read(a.ctx)
	if err != nil {
		return VaultStatus{}, err
	}
	config.Initialized = exists
	if err := saveGitHubConfig(config); err != nil {
		return VaultStatus{}, err
	}

	a.closeStoreLocked()
	clear(a.remoteEnvelope)
	a.remote, a.remoteEnvelope, a.remoteExists, a.dirty = remote, envelope, exists, false
	store, err := openMemoryStore()
	if err != nil {
		return VaultStatus{}, err
	}
	a.store = store
	initialized, err := store.isInitialized()
	if err != nil {
		return VaultStatus{}, err
	}
	initialized = initialized || exists
	return VaultStatus{
		Configured:   true,
		HasToken:     true,
		Initialized:  initialized || exists,
		RemoteExists: exists,
		Config:       config,
	}, nil
}

func (a *App) connectGitHubLocked(config GitHubConfig, token string) error {
	if a.remote != nil && a.store != nil && a.remote.config == config {
		return nil
	}
	remote := newGitHubVault(config, token)
	envelope, exists, err := remote.read(a.ctx)
	if err != nil {
		return err
	}
	store, err := openMemoryStore()
	if err != nil {
		return err
	}
	a.closeStoreLocked()
	clear(a.remoteEnvelope)
	a.remote = remote
	a.remoteEnvelope = envelope
	a.remoteExists = exists
	a.dirty = false
	a.store = store
	return nil
}

func (a *App) SetupVault(password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if utf8.RuneCountInString(password) < 8 {
		return errors.New("la contraseña maestra debe tener al menos 8 caracteres")
	}
	if err := a.requireRemoteReadyLocked(); err != nil {
		return err
	}
	if a.remoteExists {
		return errors.New("ya existe una bóveda en GitHub; desbloquéala en lugar de crear otra")
	}
	if err := a.store.initialize(password); err != nil {
		return err
	}
	a.dirty = true
	if err := savePortableToken(a.remote.config, password, a.remote.token); err != nil {
		return err
	}
	return a.syncLocked()
}

func (a *App) UnlockVault(password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.remote == nil {
		config, err := loadGitHubConfig()
		if err != nil {
			return err
		}
		token, err := loadPortableToken(config, password)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			token, err = readGitHubToken(config)
			if err != nil {
				return err
			}
			if token == "" {
				return errors.New("no hay un token disponible; vuelve a conectar tu repositorio de GitHub")
			}
		}
		if err := a.connectGitHubLocked(config, token); err != nil {
			return err
		}
	}
	if err := a.requireRemoteReadyLocked(); err != nil {
		return err
	}
	if !a.remoteExists {
		return errors.New("todavía no hay una bóveda en GitHub; crea una nueva o importa una base existente")
	}
	database, _, err := decryptVaultArchive(password, a.remoteEnvelope)
	if err != nil {
		return err
	}
	defer clear(database)
	store, err := openUninitializedMemoryStore()
	if err != nil {
		return err
	}
	a.closeStoreLocked()
	a.store = store
	if err := a.store.deserialize(database); err != nil {
		return err
	}
	unlocked, err := a.store.unlock(password)
	if err != nil {
		return err
	}
	if !unlocked {
		return errors.New("contraseña maestra incorrecta o archivo remoto dañado")
	}
	if err := savePortableToken(a.remote.config, password, a.remote.token); err != nil {
		return fmt.Errorf("bóveda desbloqueada, pero no se pudo guardar el token en el archivo portátil: %w", err)
	}
	return nil
}

func (a *App) ImportLocalVault(password string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.requireRemoteReadyLocked(); err != nil {
		return err
	}
	if a.remoteExists {
		return errors.New("ya existe una bóveda en GitHub; importar otra la sobrescribiría")
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Selecciona la base SQLite existente",
		Filters: []runtime.FileFilter{
			{DisplayName: "Base SQLite", Pattern: "*.sqlite;*.db"},
		},
	})
	if err != nil {
		return fmt.Errorf("elegir la base SQLite: %w", err)
	}
	if path == "" {
		return nil
	}
	localStore, err := openReadOnlyStore(path)
	if err != nil {
		return err
	}
	database, err := localStore.serialize()
	closeErr := localStore.close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		clear(database)
		return fmt.Errorf("cerrar bóveda SQLite existente: %w", closeErr)
	}
	defer clear(database)
	store, err := openUninitializedMemoryStore()
	if err != nil {
		return err
	}
	if err := store.deserialize(database); err != nil {
		store.close()
		return err
	}
	unlocked, err := store.unlock(password)
	if err != nil {
		store.close()
		return err
	}
	if !unlocked {
		store.close()
		return errors.New("la contraseña maestra no desbloquea la base seleccionada")
	}
	a.closeStoreLocked()
	a.store = store
	a.dirty = true
	if err := savePortableToken(a.remote.config, password, a.remote.token); err != nil {
		return fmt.Errorf("la bóveda se cargó en memoria, pero no se pudo guardar el token portátil: %w", err)
	}
	if err := a.syncLocked(); err != nil {
		return fmt.Errorf("la bóveda se cargó solo en memoria, pero no se pudo subir a GitHub; no cierres la aplicación y vuelve a intentar: %w", err)
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
	account, err := a.store.addAccount(name)
	if err != nil {
		return Account{}, err
	}
	if err := a.syncAfterMutationLocked(); err != nil {
		return account, err
	}
	return account, nil
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
	if err := a.syncAfterMutationLocked(); err != nil {
		return ImportResult{AccountName: account.Name, Count: len(entries)}, err
	}
	return ImportResult{AccountName: account.Name, Count: len(entries)}, nil
}

func (a *App) ExportCSV(accountID int64) (bool, error) {
	a.mu.Lock()
	if err := a.requireUnlockedLocked(); err != nil {
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
	if err := a.requireUnlockedLocked(); err != nil {
		return err
	}
	if err := a.store.deletePassword(id); err != nil {
		return err
	}
	return a.syncAfterMutationLocked()
}

func (a *App) SyncVault() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.requireUnlockedLocked(); err != nil {
		return err
	}
	if !a.dirty {
		return nil
	}
	return a.syncLocked()
}

func (a *App) requireRemoteReadyLocked() error {
	if a.remote == nil || a.store == nil {
		return errors.New("configura y conecta el repositorio privado de GitHub primero")
	}
	return nil
}

func (a *App) requireUnlockedLocked() error {
	if err := a.requireRemoteReadyLocked(); err != nil {
		return err
	}
	if len(a.store.key) == 0 {
		return errors.New("la bóveda está bloqueada")
	}
	return nil
}

func (a *App) syncAfterMutationLocked() error {
	a.dirty = true
	if err := a.syncLocked(); err != nil {
		return fmt.Errorf("cambios conservados solo en memoria; GitHub no confirmó el guardado. No cierres la aplicación y vuelve a intentar: %w", err)
	}
	return nil
}

func (a *App) syncLocked() error {
	if !a.dirty {
		return nil
	}
	if err := a.requireUnlockedLocked(); err != nil {
		return err
	}
	database, err := a.store.serialize()
	if err != nil {
		return err
	}
	defer clear(database)
	salt, err := a.store.vaultSalt()
	if err != nil {
		return err
	}
	envelope, err := encryptVaultArchive(a.store.key, salt, database)
	if err != nil {
		return err
	}
	if err := a.remote.write(a.ctx, envelope); err != nil {
		clear(envelope)
		return err
	}
	clear(a.remoteEnvelope)
	a.remoteEnvelope = envelope
	a.remoteExists = true
	config := a.remote.config
	config.Initialized = true
	if err := saveGitHubConfig(config); err != nil {
		return fmt.Errorf("GitHub guardó la bóveda, pero no se pudo actualizar la configuración local: %w", err)
	}
	a.remote.config = config
	a.dirty = false
	return nil
}
