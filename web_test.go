package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebHandlerServesFrontend(t *testing.T) {
	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		t.Fatal(err)
	}
	handler := newWebHandler(NewApp(), http.FS(assets))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "Password Admin") {
		t.Fatal("GET / did not return the Password Admin interface")
	}
}

func TestWebHandlerRejectsCrossOriginAPIRequests(t *testing.T) {
	handler := newWebHandler(NewApp(), http.FS(frontend))
	request := httptest.NewRequest(http.MethodPost, "/api/vault-status", strings.NewReader("{}"))
	request.Host = "localhost:45678"
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin API status = %d, want %d", response.Code, http.StatusForbidden)
	}
}
