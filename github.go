package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "PasswordAdmin"
	vaultFilePath  = "password-admin.vault"
)

type GitHubConfig struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	Initialized bool   `json:"initialized,omitempty"`
}

type githubVault struct {
	config GitHubConfig
	token  string
	sha    string
	exists bool
	client *http.Client
}

type githubContent struct {
	SHA     string `json:"sha"`
	Content string `json:"content"`
}

func configFilePath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("localizar la carpeta de la aplicación: %w", err)
	}
	return filepath.Join(filepath.Dir(executable), "password-admin.json"), nil
}

func legacyConfigFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("localizar configuración anterior de GitHub: %w", err)
	}
	return filepath.Join(configDir, "PasswordAdmin", "github.json"), nil
}

func loadGitHubConfig() (GitHubConfig, error) {
	path, err := configFilePath()
	if err != nil {
		return GitHubConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			legacyPath, legacyErr := legacyConfigFilePath()
			if legacyErr != nil {
				return GitHubConfig{}, legacyErr
			}
			data, err = os.ReadFile(legacyPath)
			if errors.Is(err, os.ErrNotExist) {
				return GitHubConfig{}, nil
			}
			if err != nil {
				return GitHubConfig{}, fmt.Errorf("leer configuración anterior de GitHub: %w", err)
			}
			var legacyConfig GitHubConfig
			if err := json.Unmarshal(data, &legacyConfig); err != nil {
				return GitHubConfig{}, fmt.Errorf("interpretar configuración anterior de GitHub: %w", err)
			}
			if err := saveGitHubConfig(legacyConfig); err != nil {
				return GitHubConfig{}, err
			}
			return legacyConfig, nil
		}
		return GitHubConfig{}, fmt.Errorf("leer configuración de GitHub: %w", err)
	}
	var config GitHubConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return GitHubConfig{}, fmt.Errorf("interpretar configuración de GitHub: %w", err)
	}
	return config, nil
}

func saveGitHubConfig(config GitHubConfig) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("crear carpeta de configuración de GitHub: %w", err)
	}
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("serializar configuración de GitHub: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("guardar configuración de GitHub: %w", err)
	}
	return nil
}

func githubTokenKey(config GitHubConfig) string {
	return strings.ToLower(config.Owner + "/" + config.Repo)
}

func newGitHubVault(config GitHubConfig, token string) *githubVault {
	return &githubVault{
		config: config,
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (g *githubVault) endpoint(path string) string {
	return "https://api.github.com/repos/" +
		url.PathEscape(g.config.Owner) + "/" + url.PathEscape(g.config.Repo) +
		"/contents/" + path
}

func (g *githubVault) request(ctx context.Context, method, endpoint string, body []byte) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("preparar solicitud a GitHub: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+g.token)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "PasswordAdmin")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := g.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("conectar con GitHub: %w", err)
	}
	return response, nil
}

func (g *githubVault) read(ctx context.Context) ([]byte, bool, error) {
	repositoryURL := "https://api.github.com/repos/" +
		url.PathEscape(g.config.Owner) + "/" + url.PathEscape(g.config.Repo)
	repositoryResponse, err := g.request(ctx, http.MethodGet, repositoryURL, nil)
	if err != nil {
		return nil, false, err
	}
	if repositoryResponse.StatusCode != http.StatusOK {
		return nil, false, githubResponseError("validar el repositorio privado", repositoryResponse)
	}
	if err := closeResponse(repositoryResponse); err != nil {
		return nil, false, err
	}

	response, err := g.request(ctx, http.MethodGet, g.endpoint(vaultFilePath), nil)
	if err != nil {
		return nil, false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		g.exists, g.sha = false, ""
		return nil, false, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, false, githubResponseError("descargar la bóveda", response)
	}
	var content githubContent
	if err := json.NewDecoder(response.Body).Decode(&content); err != nil {
		return nil, false, fmt.Errorf("interpretar respuesta de GitHub: %w", err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(content.Content), ""))
	if err != nil {
		return nil, false, fmt.Errorf("decodificar archivo de bóveda de GitHub: %w", err)
	}
	if content.SHA == "" {
		return nil, false, errors.New("GitHub no devolvió la versión del archivo de bóveda")
	}
	g.exists, g.sha = true, content.SHA
	return data, true, nil
}

func closeResponse(response *http.Response) error {
	_, copyErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if copyErr != nil {
		return fmt.Errorf("leer respuesta de GitHub: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("cerrar respuesta de GitHub: %w", closeErr)
	}
	return nil
}

func githubResponseError(action string, response *http.Response) error {
	defer response.Body.Close()
	var detail struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&detail)
	if detail.Message == "" {
		return fmt.Errorf("GitHub no pudo %s (HTTP %d)", action, response.StatusCode)
	}
	return fmt.Errorf("GitHub no pudo %s (HTTP %d): %s", action, response.StatusCode, detail.Message)
}

func (g *githubVault) write(ctx context.Context, contents []byte) error {
	payload := struct {
		Message string `json:"message"`
		Content string `json:"content"`
		SHA     string `json:"sha,omitempty"`
	}{
		Message: "Update encrypted Password Admin vault",
		Content: base64.StdEncoding.EncodeToString(contents),
		SHA:     g.sha,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("preparar actualización de la bóveda: %w", err)
	}
	response, err := g.request(ctx, http.MethodPut, g.endpoint(vaultFilePath), body)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusConflict {
		return errors.New("la bóveda cambió en GitHub desde que se abrió; se evitó sobrescribir la versión remota")
	}
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return githubResponseError("guardar la bóveda", response)
	}
	var result struct {
		Content githubContent `json:"content"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("confirmar actualización de GitHub: %w", err)
	}
	if result.Content.SHA == "" {
		return errors.New("GitHub no confirmó la nueva versión de la bóveda")
	}
	g.exists, g.sha = true, result.Content.SHA
	return nil
}

func validateGitHubSettings(owner, repo, token string) (GitHubConfig, error) {
	config := GitHubConfig{
		Owner: strings.TrimSpace(owner),
		Repo:  strings.TrimSpace(repo),
	}
	if config.Owner == "" || config.Repo == "" {
		return GitHubConfig{}, errors.New("indica el propietario y el nombre del repositorio privado")
	}
	if strings.ContainsAny(config.Owner+config.Repo, "/?#") {
		return GitHubConfig{}, errors.New("el propietario y el repositorio deben ser nombres, no una URL")
	}
	if strings.TrimSpace(token) == "" {
		return GitHubConfig{}, errors.New("ingresa un token fine-grained de GitHub")
	}
	return config, nil
}

func readGitHubToken(config GitHubConfig) (string, error) {
	token, err := keyring.Get(keyringService, githubTokenKey(config))
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("leer el token del almacén seguro del sistema: %w", err)
	}
	return token, nil
}
