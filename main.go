package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var frontend embed.FS

func main() {
	databasePath, err := configuredDatabasePath()
	if err != nil {
		log.Fatal(err)
	}
	application, err := NewApp(databasePath)
	if err != nil {
		log.Fatalf("abrir base de datos local %q: %v", databasePath, err)
	}
	defer application.shutdown()

	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	if err := wails.Run(&options.App{
		Title:     "Password Admin",
		Width:     1280,
		Height:    820,
		MinWidth:  960,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  application.startup,
		OnShutdown: func(context.Context) { application.shutdown() },
		Bind:       []interface{}{application},
	}); err != nil {
		log.Fatalf("iniciar ventana de escritorio: %v", err)
	}
}

func configuredDatabasePath() (string, error) {
	databasePath, err := configuredValue("DB_PATH")
	if err != nil {
		return "", err
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("localizar el ejecutable para la base de datos: %w", err)
	}
	if databasePath != "" {
		if filepath.IsAbs(databasePath) {
			return databasePath, nil
		}
		return filepath.Join(filepath.Dir(executable), databasePath), nil
	}
	return filepath.Join(filepath.Dir(executable), "data", "password-admin.sqlite"), nil
}

func configuredValue(key string) (string, error) {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("localizar el ejecutable para leer .env: %w", err)
	}
	path := filepath.Join(filepath.Dir(executable), ".env")
	return readDotEnvValue(path, key)
}

func readDotEnvValue(path, key string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("leer %s: %w", path, err)
	}
	return parseEnvValue(data, key), nil
}

func parseEnvValue(data []byte, key string) string {
	var value string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "export ")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, envValue, found := strings.Cut(line, "=")
		if found && strings.TrimSpace(name) == key {
			value = strings.Trim(strings.TrimSpace(envValue), `"'`)
		}
	}
	return value
}
