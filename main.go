package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//go:embed all:frontend
var frontend embed.FS

const defaultPort = "45678"

func main() {
	port, err := configuredPort()
	if err != nil {
		log.Fatal(err)
	}
	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	databasePath, err := configuredDatabasePath()
	if err != nil {
		log.Fatal(err)
	}
	application, err := NewApp(databasePath)
	if err != nil {
		log.Fatalf("abrir base de datos local %q: %v", databasePath, err)
	}
	addr := net.JoinHostPort("127.0.0.1", port)
	server := &http.Server{
		Addr:              addr,
		Handler:           newWebHandler(application, http.FS(assets)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		log.Fatalf("iniciar servidor web en %s: %v", server.Addr, err)
	}
	log.Printf("Password Admin disponible en http://%s", listener.Addr())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("servidor web: %v", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("cerrar servidor web: %v", err)
		}
		cancel()
	}
	application.shutdown()
}

func configuredPort() (string, error) {
	port, err := configuredValue("PORT")
	if err != nil {
		return "", err
	}
	if port == "" {
		port = defaultPort
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", fmt.Errorf("PORT debe ser un número entre 1 y 65535")
	}
	return strconv.Itoa(number), nil
}

func configuredDatabasePath() (string, error) {
	databasePath, err := configuredValue("DB_PATH")
	if err != nil {
		return "", err
	}
	if databasePath != "" {
		return databasePath, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("localizar el ejecutable para la base de datos: %w", err)
	}
	return filepath.Join(filepath.Dir(executable), "password-admin.sqlite"), nil
}

func configuredValue(key string) (string, error) {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("localizar el ejecutable para leer .env: %w", err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("localizar la carpeta actual para leer .env: %w", err)
	}
	paths := []string{
		filepath.Join(workingDirectory, ".env"),
		filepath.Join(filepath.Dir(executable), ".env"),
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("leer %s: %w", path, err)
		}
		if value := parseEnvValue(data, key); value != "" {
			return value, nil
		}
	}
	return "", nil
}

func parsePortFromEnv(data []byte) string {
	return parseEnvValue(data, "PORT")
}

func parseEnvValue(data []byte, key string) string {
	var port string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "export ")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, "=")
		if found && strings.TrimSpace(name) == key {
			port = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return port
}
