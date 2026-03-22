package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// Scope for read-only access to Meet conference records and participants.
	meetReadOnlyScope = "https://www.googleapis.com/auth/meetings.space.readonly"
)

// getOAuthClient returns an HTTP client authenticated with Google OAuth2.
//
// It expects a credentials.json file (OAuth 2.0 Client ID downloaded from the
// Google Cloud Console) in the current directory or at the path specified by
// GOOGLE_CREDENTIALS_FILE.
//
// On first run it opens a browser-based consent flow and caches the resulting
// token in ~/.meet-attendees-token.json for subsequent runs.
func getOAuthClient(ctx context.Context) (*http.Client, error) {
	credFile := os.Getenv("GOOGLE_CREDENTIALS_FILE")
	if credFile == "" {
		credFile = "credentials.json"
	}

	b, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to read client secret file %q: %w\n\n"+
				"Download OAuth 2.0 Client ID credentials from:\n"+
				"  https://console.cloud.google.com/apis/credentials\n"+
				"and save as credentials.json in the working directory,\n"+
				"or set GOOGLE_CREDENTIALS_FILE to its path", credFile, err)
	}

	config, err := google.ConfigFromJSON(b, meetReadOnlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret: %w", err)
	}

	tok, err := tokenFromCache()
	if err != nil {
		tok, err = tokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		if err := saveToken(tok); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not cache token: %v\n", err)
		}
	}

	return config.Client(ctx, tok), nil
}

func tokenCachePath() string {
	if p := os.Getenv("TOKEN_CACHE_FILE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".meet-attendees-token.json")
}

func tokenFromCache() (*oauth2.Token, error) {
	f, err := os.Open(tokenCachePath())
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	return tok, json.NewDecoder(f).Decode(tok)
}

func saveToken(token *oauth2.Token) error {
	path := tokenCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

// callbackURL returns the OAuth redirect URL, preferring the
// OAUTH_CALLBACK_URL environment variable over the default.
func callbackURL() string {
	if u := os.Getenv("OAUTH_CALLBACK_URL"); u != "" {
		return u
	}
	return "http://localhost:8085/callback"
}

// tokenFromWeb obtains an OAuth token via the browser consent flow.
//
// For local callback URLs (localhost / 127.0.0.1) it starts a temporary HTTP
// server on the callback port to receive the code.  For remote URLs the code
// is delivered to the /callback route on the main web server and forwarded
// here via oauthCodeCh.
func tokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	config.RedirectURL = callbackURL()

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Open this URL in your browser to authorize:\n\n  %s\n\nWaiting for authorization...\n", authURL)

	code, err := receiveCode(config.RedirectURL)
	if err != nil {
		return nil, err
	}

	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange authorization code: %w", err)
	}
	return tok, nil
}

// receiveCode waits for the OAuth authorization code.
// For local redirect URLs it spins up a temporary HTTP server; for remote
// URLs it reads from oauthCodeCh (populated by the main server's /callback).
func receiveCode(redirectURL string) (string, error) {
	listenAddr, err := parseListenAddr(redirectURL)
	if err != nil {
		// Non-local URL: the main server handles /callback.
		return <-oauthCodeCh, nil
	}

	// Local URL: start a dedicated callback server.
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no code in callback: %s", r.URL.String())
			fmt.Fprintln(w, "Error: no authorization code received.")
			return
		}
		codeCh <- code
		fmt.Fprintln(w, "Authorization successful! You can close this tab.")
	})

	srv := &http.Server{Addr: listenAddr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		srv.Close()
		return "", fmt.Errorf("auth callback error: %w", err)
	}

	if err := srv.Close(); err != nil {
		return "", fmt.Errorf("failed to close callback server: %w", err)
	}
	return code, nil
}

// parseListenAddr extracts a ":port" listen address from a full callback URL.
// It only returns a result for localhost/127.0.0.1 URLs so that remote tunnel
// URLs (e.g. ngrok) don't accidentally bind on a remote port.
func parseListenAddr(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	if host != "localhost" && host != "127.0.0.1" {
		return "", fmt.Errorf("non-local callback URL %q: keeping default listen address", rawURL)
	}
	port := u.Port()
	if port == "" {
		port = "8085"
	}
	return ":" + port, nil
}
