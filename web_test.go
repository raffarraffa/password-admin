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
	if !strings.Contains(response.Body.String(), `<base href="/">`) {
		t.Fatal("GET / did not set the document base to the domain root")
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

func TestWebHandlerServesFrontendUnderBasePath(t *testing.T) {
	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		t.Fatal(err)
	}
	handler := newWebHandler(NewApp(), http.FS(assets), "/pass-admin")

	redirect := httptest.NewRecorder()
	handler.ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "/pass-admin", nil))
	if redirect.Code != http.StatusPermanentRedirect || redirect.Header().Get("Location") != "/pass-admin/" {
		t.Fatalf("GET /pass-admin = (%d, %q), want redirect to /pass-admin/",
			redirect.Code, redirect.Header().Get("Location"))
	}

	for _, path := range []string{"/pass-admin/", "/pass-admin/app.css", "/pass-admin/app.js"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusOK)
		}
		if path == "/pass-admin/" &&
			!strings.Contains(response.Body.String(), `<base href="/pass-admin/">`) {
			t.Error("GET /pass-admin/ did not set the subpath as the document base")
		}
	}

	for _, path := range []string{"/", "/app.css", "/app.js"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Errorf("GET %s (prefix stripped by proxy) status = %d, want %d",
				path, response.Code, http.StatusOK)
		}
	}

	apiRequest := httptest.NewRequest(http.MethodPost, "/pass-admin/api/setup-vault",
		strings.NewReader(`{"password":"test-password"}`))
	apiRequest.Host = "password.example"
	apiRequest.Header.Set("Origin", "https://password.example")
	apiResponse := httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, apiRequest)
	if apiResponse.Code != http.StatusUnprocessableEntity {
		t.Errorf("POST /pass-admin/api/setup-vault status = %d, want %d: %s",
			apiResponse.Code, http.StatusUnprocessableEntity, apiResponse.Body.String())
	}

	strippedAPIRequest := httptest.NewRequest(http.MethodPost, "/api/setup-vault",
		strings.NewReader(`{"password":"test-password"}`))
	strippedAPIRequest.Host = "password.example"
	strippedAPIRequest.Header.Set("Origin", "https://password.example")
	strippedAPIResponse := httptest.NewRecorder()
	handler.ServeHTTP(strippedAPIResponse, strippedAPIRequest)
	if strippedAPIResponse.Code != http.StatusUnprocessableEntity {
		t.Errorf("POST /api/setup-vault (prefix stripped by proxy) status = %d, want %d: %s",
			strippedAPIResponse.Code, http.StatusUnprocessableEntity, strippedAPIResponse.Body.String())
	}

	outsidePath := httptest.NewRecorder()
	handler.ServeHTTP(outsidePath, httptest.NewRequest(http.MethodGet, "/outside/", nil))
	if outsidePath.Code != http.StatusNotFound {
		t.Errorf("GET outside base path status = %d, want %d", outsidePath.Code, http.StatusNotFound)
	}
}
