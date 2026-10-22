package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebHandlerServesFrontend(t *testing.T) {
	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(filepath.Join(t.TempDir(), "vault.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.shutdown()
	handler := newWebHandler(app, http.FS(assets))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "Password Admin") {
		t.Fatal("GET / did not return the Password Admin interface")
	}
	if !strings.Contains(response.Body.String(), `href="./app.css"`) ||
		!strings.Contains(response.Body.String(), `src="./app.js"`) {
		t.Fatal("frontend assets must use paths relative to the application URL")
	}
	backend, err := frontend.ReadFile("frontend/backend.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(backend), "fetch(`./api/") {
		t.Fatal("frontend API requests must use paths relative to the application URL")
	}
}

func TestWebHandlerRejectsCrossOriginAPIRequests(t *testing.T) {
	app, err := NewApp(filepath.Join(t.TempDir(), "vault.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.shutdown()
	handler := newWebHandler(app, http.FS(frontend))
	request := httptest.NewRequest(http.MethodPost, "/api/vault-status", strings.NewReader("{}"))
	request.Host = "localhost:45678"
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin API status = %d, want %d", response.Code, http.StatusForbidden)
	}
}
