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

// tokenFromWeb runs a local callback server and opens the consent URL.
func tokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	config.RedirectURL = callbackURL()

	// Derive the listen address from the redirect URL path/port.
	// If the URL is non-local (e.g. a tunnel) the local server still binds on
	// the default port so the tunnel can forward to it.
	listenAddr := ":8085"
	if u := os.Getenv("OAUTH_CALLBACK_URL"); u != "" {
		// Parse host:port from the env URL when it is localhost-like.
		parsed, err := parseListenAddr(u)
		if err == nil {
			listenAddr = parsed
		}
	}

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

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Open this URL in your browser to authorize:\n\n  %s\n\nWaiting for authorization...\n", authURL)

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		srv.Close()
		return nil, fmt.Errorf("auth callback error: %w", err)
	}

	err := srv.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close server: %w", err)
	}

	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange authorization code: %w", err)
	}
	return tok, nil
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
