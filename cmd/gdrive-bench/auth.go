package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gdrive-bench")
}

func tokenPath() string {
	return filepath.Join(configDir(), "token.json")
}

func credentialsPath() string {
	if p := os.Getenv("GDRIVE_CREDENTIALS"); p != "" {
		return p
	}
	return filepath.Join(configDir(), "credentials.json")
}

func folderID() string {
	return os.Getenv("GDRIVE_FOLDER")
}

func oauthConfig() (*oauth2.Config, error) {
	b, err := os.ReadFile(credentialsPath())
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w (set GDRIVE_CREDENTIALS or place credentials.json in %s)", err, configDir())
	}
	// drive.file scope: only access files created/opened by this app
	return google.ConfigFromJSON(b, drive.DriveFileScope)
}

func getClient() (*http.Client, error) {
	cfg, err := oauthConfig()
	if err != nil {
		return nil, err
	}

	tok, err := loadToken()
	if err != nil {
		return nil, fmt.Errorf("not authenticated — run 'gdrive-bench auth' first: %w", err)
	}

	return cfg.Client(context.Background(), tok), nil
}

func loadToken() (*oauth2.Token, error) {
	f, err := os.Open(tokenPath())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var tok oauth2.Token
	return &tok, json.NewDecoder(f).Decode(&tok)
}

func saveToken(tok *oauth2.Token) error {
	if err := os.MkdirAll(configDir(), 0700); err != nil {
		return err
	}
	f, err := os.Create(tokenPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(tok)
}

func runAuth() error {
	cfg, err := oauthConfig()
	if err != nil {
		return err
	}

	// Use a local redirect for the OAuth flow
	cfg.RedirectURL = "http://localhost:8085/callback"

	authURL := cfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Open this URL in your browser:\n\n%s\n\n", authURL)

	codeCh := make(chan string, 1)
	srv := &http.Server{Addr: ":8085"}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		codeCh <- r.URL.Query().Get("code")
		fmt.Fprint(w, "<h1>Authenticated! You can close this tab.</h1>")
	})

	go srv.ListenAndServe()

	code := <-codeCh
	srv.Close()

	tok, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("exchange token: %w", err)
	}

	if err := saveToken(tok); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	fmt.Println(`{"status": "authenticated"}`)
	return nil
}

func driveService() (*drive.Service, error) {
	client, err := getClient()
	if err != nil {
		return nil, err
	}
	return drive.New(client)
}
