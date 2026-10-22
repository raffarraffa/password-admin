package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	maxJSONRequestSize = 16 << 20
	maxVaultUploadSize = 64 << 20
)

type webHandler struct {
	app      *App
	assets   http.FileSystem
	static   http.Handler
	basePath string
}

func newWebHandler(app *App, static http.FileSystem, basePaths ...string) http.Handler {
	basePath := ""
	if len(basePaths) > 0 {
		basePath = basePaths[0]
	}
	return &webHandler{app: app, assets: static, static: http.FileServer(static), basePath: basePath}
}

func normalizeBasePath(basePath string) (string, error) {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" || basePath == "/" {
		return "", nil
	}
	basePath = strings.TrimSuffix(basePath, "/")
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	if strings.ContainsAny(basePath, `?#%\\<>"'`) || strings.Contains(basePath, "//") ||
		path.Clean(basePath) != basePath {
		return "", fmt.Errorf("BASE_PATH debe ser una ruta absoluta limpia, por ejemplo /pass-admin")
	}
	return basePath, nil
}

func (h *webHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")

	if h.basePath != "" {
		if r.URL.Path == h.basePath {
			location := h.basePath + "/"
			if r.URL.RawQuery != "" {
				location += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, location, http.StatusPermanentRedirect)
			return
		}
		if strings.HasPrefix(r.URL.Path, h.basePath+"/") {
			r2 := r.Clone(r.Context())
			urlCopy := *r.URL
			urlCopy.Path = strings.TrimPrefix(r.URL.Path, h.basePath)
			r2.URL = &urlCopy
			r = r2
		}
	}

	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		h.serveIndex(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		h.serveAPI(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}
	h.static.ServeHTTP(w, r)
}

func (h *webHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}
	file, err := h.assets.Open("index.html")
	if err != nil {
		http.Error(w, "no se pudo cargar la interfaz", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "no se pudo leer la interfaz", http.StatusInternalServerError)
		return
	}
	baseHref := h.basePath + "/"
	if h.basePath == "" {
		baseHref = "/"
	}
	content = bytes.ReplaceAll(content, []byte("{{BASE_HREF}}"), []byte(baseHref))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(content))
}

func (h *webHandler) serveAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
		return
	}
	if err := validateRequestOrigin(r); err != nil {
		writeAPIError(w, http.StatusForbidden, err)
		return
	}

	switch strings.TrimPrefix(r.URL.Path, "/api/") {
	case "vault-status":
		h.handleJSON(w, r, func() (any, error) { return h.app.VaultStatus() })
	case "configure-github":
		h.handleJSON(w, r, func(body apiRequest) (any, error) {
			return h.app.ConfigureGitHub(body.Owner, body.Repo, body.Token)
		})
	case "setup-vault":
		h.handleJSON(w, r, func(body apiRequest) (any, error) {
			return nil, h.app.SetupVault(body.Password)
		})
	case "unlock-vault":
		h.handleJSON(w, r, func(body apiRequest) (any, error) {
			return nil, h.app.UnlockVault(body.Password)
		})
	case "accounts":
		h.handleJSON(w, r, func() (any, error) { return h.app.Accounts() })
	case "create-account":
		h.handleJSON(w, r, func(body apiRequest) (any, error) {
			return h.app.CreateAccount(body.Name)
		})
	case "passwords":
		h.handleJSON(w, r, func(body apiRequest) (any, error) {
			return h.app.Passwords(body.AccountID)
		})
	case "password-details":
		h.handleJSON(w, r, func(body apiRequest) (any, error) {
			return h.app.PasswordDetails(body.EntryID)
		})
	case "preview-csv":
		h.handleJSON(w, r, func(body apiRequest) (any, error) {
			return h.app.PreviewCSV(body.Content)
		})
	case "import-csv":
		h.handleJSON(w, r, func(body apiRequest) (any, error) {
			return h.app.ImportCSV(body.Content, body.AccountID, body.NewAccountName)
		})
	case "export-csv":
		h.exportCSV(w, r)
	case "delete-password":
		h.handleJSON(w, r, func(body apiRequest) (any, error) {
			return nil, h.app.DeletePassword(body.EntryID)
		})
	case "sync-vault":
		h.handleJSON(w, r, func() (any, error) {
			return nil, h.app.SyncVault()
		})
	case "import-local-vault":
		h.importLocalVault(w, r)
	default:
		http.NotFound(w, r)
	}
}

type apiRequest struct {
	Owner          string `json:"owner"`
	Repo           string `json:"repo"`
	Token          string `json:"token"`
	Password       string `json:"password"`
	Name           string `json:"name"`
	AccountID      int64  `json:"accountID"`
	EntryID        int64  `json:"entryID"`
	Content        string `json:"content"`
	NewAccountName string `json:"newAccountName"`
}

func (h *webHandler) handleJSON(w http.ResponseWriter, r *http.Request, operation any) {
	body := apiRequest{}
	if r.ContentLength != 0 {
		r.Body = http.MaxBytesReader(w, r.Body, maxJSONRequestSize)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, fmt.Errorf("solicitud JSON inválida: %w", err))
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeAPIError(w, http.StatusBadRequest, errors.New("la solicitud debe contener un único objeto JSON"))
			return
		}
	}
	var (
		result any
		err    error
	)
	switch action := operation.(type) {
	case func() (any, error):
		result, err = action()
	case func(apiRequest) (any, error):
		result, err = action(body)
	default:
		writeAPIError(w, http.StatusInternalServerError, errors.New("operación web no válida"))
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusUnprocessableEntity, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		return
	}
}

func (h *webHandler) importLocalVault(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxVaultUploadSize)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeAPIError(w, http.StatusBadRequest, fmt.Errorf("formulario de importación inválido: %w", err))
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	password := r.FormValue("password")
	file, _, err := r.FormFile("vault")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, fmt.Errorf("falta seleccionar el archivo SQLite: %w", err))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, fmt.Errorf("leer archivo SQLite: %w", err))
		return
	}
	defer clear(data)
	if err := h.app.ImportLocalVault(password, data); err != nil {
		writeAPIError(w, http.StatusUnprocessableEntity, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = io.WriteString(w, "null\n")
}

func (h *webHandler) exportCSV(w http.ResponseWriter, r *http.Request) {
	var body apiRequest
	if err := decodeJSONRequest(w, r, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	data, err := h.app.ExportCSV(body.AccountID)
	if err != nil {
		writeAPIError(w, http.StatusUnprocessableEntity, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": "chrome-passwords.csv",
	}))
	_, _ = w.Write(data)
	clear(data)
}

func decodeJSONRequest(w http.ResponseWriter, r *http.Request, body *apiRequest) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONRequestSize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(body); err != nil {
		return fmt.Errorf("solicitud JSON inválida: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("la solicitud debe contener un único objeto JSON")
	}
	return nil
}

func validateRequestOrigin(r *http.Request) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Path != "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		!strings.EqualFold(parsed.Host, r.Host) {
		return errors.New("el origen de la solicitud no coincide con el servidor")
	}
	return nil
}

func writeAPIError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
