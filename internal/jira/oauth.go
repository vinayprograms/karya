package jira

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// TokenStore manages OAuth2 tokens for a named JIRA connection,
// persisting them to ~/.config/karya/jira-tokens/<name>.json.
type TokenStore struct {
	name string
	path string
	mu   sync.Mutex
	tok  *oauth2.Token
	cfg  *oauth2.Config
}

// NewTokenStore creates a token store for the given connection name.
func NewTokenStore(name string) *TokenStore {
	home, _ := os.UserHomeDir()
	return &TokenStore{
		name: name,
		path: filepath.Join(home, ".config", "karya", "jira-tokens", name+".json"),
	}
}

// Load reads the stored token from disk.
func (ts *TokenStore) Load() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	data, err := os.ReadFile(ts.path)
	if err != nil {
		return err
	}
	var stored storedToken
	if err := json.Unmarshal(data, &stored); err != nil {
		return fmt.Errorf("parse token file: %w", err)
	}
	ts.tok = stored.Token
	ts.cfg = stored.OAuthConfig
	return nil
}

// HasToken returns true if a token is loaded.
func (ts *TokenStore) HasToken() bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.tok != nil
}

// AccessToken returns a valid access token, refreshing if needed.
func (ts *TokenStore) AccessToken() (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.tok == nil {
		return "", fmt.Errorf("no token stored for %q; run 'todo jira-auth %s'", ts.name, ts.name)
	}

	if ts.tok.Expiry.After(time.Now().Add(30 * time.Second)) {
		return ts.tok.AccessToken, nil
	}

	if ts.cfg == nil {
		return "", fmt.Errorf("no OAuth config stored; re-run 'todo jira-auth %s'", ts.name)
	}

	src := ts.cfg.TokenSource(context.Background(), ts.tok)
	newTok, err := src.Token()
	if err != nil {
		return "", fmt.Errorf("refreshing token: %w", err)
	}
	ts.tok = newTok
	if err := ts.save(); err != nil {
		return "", fmt.Errorf("saving refreshed token: %w", err)
	}
	return newTok.AccessToken, nil
}

// RunAuthFlow discovers OAuth metadata from the MCP endpoint, performs
// dynamic client registration, and runs the PKCE authorization code flow.
func (ts *TokenStore) RunAuthFlow(ctx context.Context, endpoint string) error {
	httpClient := &http.Client{Timeout: 30 * time.Second}

	// Step 1: Discover authorization server metadata
	baseURL := strings.TrimSuffix(endpoint, "/mcp")
	baseURL = strings.TrimSuffix(baseURL, "/")
	// Try without path first (e.g., https://mcp.atlassian.com)
	serverBase := baseURL
	if idx := strings.Index(baseURL[8:], "/"); idx >= 0 {
		serverBase = baseURL[:8+idx]
	}

	metaURL := serverBase + "/.well-known/oauth-authorization-server"
	meta, err := fetchAuthServerMeta(ctx, httpClient, metaURL)
	if err != nil {
		return fmt.Errorf("fetching auth server metadata: %w", err)
	}

	// Step 2: Dynamic client registration
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("starting callback server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://localhost:%d/callback", port)

	clientID, err := registerClient(ctx, httpClient, meta.RegistrationEndpoint, redirectURL)
	if err != nil {
		return fmt.Errorf("dynamic client registration: %w", err)
	}

	// Step 3: PKCE authorization code flow
	cfg := &oauth2.Config{
		ClientID: clientID,
		Endpoint: oauth2.Endpoint{
			AuthURL:  meta.AuthorizationEndpoint,
			TokenURL: meta.TokenEndpoint,
		},
		RedirectURL: redirectURL,
		Scopes:      meta.ScopesSupported,
	}

	verifier := generateCodeVerifier()
	challenge := computeCodeChallenge(verifier)
	state := generateState()

	authURL := cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("state mismatch")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback: %s", r.URL.Query().Get("error"))
			http.Error(w, "No code", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "<html><body><h2>Authorization successful!</h2><p>You can close this tab.</p></body></html>")
		codeCh <- code
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(listener) }()
	defer srv.Shutdown(ctx)

	fmt.Printf("Opening browser for JIRA authorization (%s)...\n", ts.name)
	fmt.Printf("If the browser doesn't open, visit:\n%s\n", authURL)
	openBrowser(authURL)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}

	tok, err := cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return fmt.Errorf("exchanging code for token: %w", err)
	}

	ts.mu.Lock()
	ts.tok = tok
	ts.cfg = cfg
	ts.mu.Unlock()

	return ts.saveWithEndpoint(endpoint)
}

// Endpoint returns the MCP endpoint URL stored with the token.
func (ts *TokenStore) Endpoint() string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	data, err := os.ReadFile(ts.path)
	if err != nil {
		return ""
	}
	var stored storedToken
	if err := json.Unmarshal(data, &stored); err != nil {
		return ""
	}
	return stored.Endpoint
}

type storedToken struct {
	Token       *oauth2.Token  `json:"token"`
	OAuthConfig *oauth2.Config `json:"oauth_config"`
	Endpoint    string         `json:"endpoint"`
}

func (ts *TokenStore) save() error {
	return ts.saveWithEndpoint("")
}

func (ts *TokenStore) saveWithEndpoint(endpoint string) error {
	dir := filepath.Dir(ts.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	// Preserve existing endpoint if not overwriting
	if endpoint == "" {
		if existing, err := os.ReadFile(ts.path); err == nil {
			var old storedToken
			if json.Unmarshal(existing, &old) == nil {
				endpoint = old.Endpoint
			}
		}
	}
	stored := storedToken{Token: ts.tok, OAuthConfig: ts.cfg, Endpoint: endpoint}
	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	return os.WriteFile(ts.path, data, 0600)
}

type authServerMetadata struct {
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint"`
	ScopesSupported       []string `json:"scopes_supported"`
}

func fetchAuthServerMeta(ctx context.Context, client *http.Client, metaURL string) (*authServerMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata endpoint returned %d", resp.StatusCode)
	}

	var meta authServerMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func registerClient(ctx context.Context, client *http.Client, endpoint, redirectURL string) (string, error) {
	body := map[string]interface{}{
		"client_name":                  "karya-todo",
		"redirect_uris":               []string{redirectURL},
		"grant_types":                  []string{"authorization_code", "refresh_token"},
		"response_types":              []string{"code"},
		"token_endpoint_auth_method":  "none",
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("registration returned %d", resp.StatusCode)
	}

	var result struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.ClientID == "" {
		return "", fmt.Errorf("no client_id in registration response")
	}
	return result.ClientID, nil
}

func generateCodeVerifier() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func computeCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func generateState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func openBrowser(rawURL string) {
	// Validate URL before passing to shell
	if _, err := url.Parse(rawURL); err != nil {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "linux":
		cmd = exec.Command("xdg-open", rawURL)
	default:
		cmd = exec.Command("open", rawURL)
	}
	_ = cmd.Start()
}
