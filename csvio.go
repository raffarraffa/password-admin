package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
)

func readChromeCSV(source io.Reader) ([]PasswordEntry, error) {
	reader := csv.NewReader(source)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("leer encabezado CSV: %w", err)
	}
	columns := make(map[string]int, len(header))
	for i, name := range header {
		normalized := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))
		if _, exists := columns[normalized]; exists {
			return nil, fmt.Errorf("columna duplicada en el encabezado: %q", normalized)
		}
		columns[normalized] = i
	}
	urlColumn, hasURL := columns["url"]
	passwordColumn, hasPassword := columns["password"]
	if !hasURL || !hasPassword {
		return nil, errors.New("el CSV debe tener columnas \"url\" y \"password\" (exportación de Chrome)")
	}
	nameColumn, hasName := columns["name"]
	usernameColumn, hasUsername := columns["username"]

	var entries []PasswordEntry
	for line := 2; ; line++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("fila %d: %w", line, err)
		}
		if len(record) == 1 && strings.TrimSpace(record[0]) == "" {
			continue
		}
		if urlColumn >= len(record) || passwordColumn >= len(record) ||
			(hasName && nameColumn >= len(record)) || (hasUsername && usernameColumn >= len(record)) {
			return nil, fmt.Errorf("fila %d: faltan columnas", line)
		}
		entry := PasswordEntry{
			URL:      strings.TrimSpace(record[urlColumn]),
			Password: record[passwordColumn],
		}
		if hasName {
			entry.Name = strings.TrimSpace(record[nameColumn])
		}
		if hasUsername {
			entry.Username = record[usernameColumn]
		}
		if entry.URL == "" {
			return nil, fmt.Errorf("fila %d: falta la URL", line)
		}
		if entry.Name == "" {
			if parsed, err := url.Parse(entry.URL); err == nil {
				entry.Name = parsed.Hostname()
			}
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, errors.New("el CSV no contiene contraseñas para importar")
	}
	return entries, nil
}

func writeChromeCSV(destination io.Writer, entries []PasswordEntry) error {
	writer := csv.NewWriter(destination)
	if err := writer.Write([]string{"name", "url", "username", "password"}); err != nil {
		return fmt.Errorf("escribir encabezado CSV: %w", err)
	}
	for _, entry := range entries {
		if err := writer.Write([]string{entry.Name, entry.URL, entry.Username, entry.Password}); err != nil {
			return fmt.Errorf("escribir fila CSV: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("finalizar CSV: %w", err)
	}
	return nil
}
