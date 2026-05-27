package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

type appConfig struct {
	Bucket         string   `json:"bucket"`
	UserPrefix     string   `json:"user_prefix"`
	SharedPrefixes []string `json:"shared_prefixes"`
	AgentID        string   `json:"agent_id"`
	Credentials    string   `json:"credentials"`
}

var cachedConfig *appConfig

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gcs-bench")
}

func configFilePath() string {
	return filepath.Join(configDir(), "config.json")
}

func loadConfig() *appConfig {
	if cachedConfig != nil {
		return cachedConfig
	}

	cfg := &appConfig{}

	data, err := os.ReadFile(configFilePath())
	if err == nil {
		json.Unmarshal(data, cfg)
	}

	// Env vars override config file
	if v := os.Getenv("GCS_BUCKET"); v != "" {
		cfg.Bucket = v
	}
	if v := os.Getenv("GCS_PREFIX"); v != "" {
		cfg.UserPrefix = v
	}
	if v := os.Getenv("GCS_AGENT_ID"); v != "" {
		cfg.AgentID = v
	}
	if v := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); v != "" {
		cfg.Credentials = v
	}

	if cfg.AgentID == "" {
		hostname, _ := os.Hostname()
		user := os.Getenv("USER")
		if user == "" {
			user = "unknown"
		}
		cfg.AgentID = fmt.Sprintf("%s@%s", user, hostname)
	}

	cachedConfig = cfg
	return cfg
}

func saveConfig(cfg *appConfig) error {
	if err := os.MkdirAll(configDir(), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFilePath(), data, 0600)
}

func bucketName() string {
	return loadConfig().Bucket
}

func prefix() string {
	p := loadConfig().UserPrefix
	if p != "" && p[len(p)-1] != '/' {
		p += "/"
	}
	return p
}

func objectKey(name string) string {
	return prefix() + name
}

func credentialsPath() string {
	cfg := loadConfig()
	if cfg.Credentials != "" {
		expanded := cfg.Credentials
		if strings.HasPrefix(expanded, "~/") {
			home, _ := os.UserHomeDir()
			expanded = filepath.Join(home, expanded[2:])
		}
		return expanded
	}
	defaultSA := filepath.Join(configDir(), "sa-key.json")
	if _, err := os.Stat(defaultSA); err == nil {
		return defaultSA
	}
	return ""
}

func gcsClient(ctx context.Context) (*storage.Client, error) {
	b := bucketName()
	if b == "" {
		return nil, fmt.Errorf("bucket not configured — run 'gcs-bench setup' or set GCS_BUCKET")
	}

	saKeyPath := credentialsPath()
	if saKeyPath != "" {
		return storage.NewClient(ctx, option.WithCredentialsFile(saKeyPath))
	}

	return storage.NewClient(ctx)
}

func bucket(client *storage.Client) *storage.BucketHandle {
	return client.Bucket(bucketName())
}

// --- Permission validation ---

type accessResult struct {
	allowed bool
	reason  string
}

func validateAccess(key string, operation string) accessResult {
	cfg := loadConfig()

	if cfg.UserPrefix == "" {
		return accessResult{allowed: true}
	}

	userPrefix := cfg.UserPrefix
	if !strings.HasSuffix(userPrefix, "/") {
		userPrefix += "/"
	}

	// Strip full bucket prefix if present to get relative key
	relKey := key
	if strings.HasPrefix(relKey, userPrefix) {
		return accessResult{allowed: true}
	}

	for _, shared := range cfg.SharedPrefixes {
		sp := shared
		if !strings.HasSuffix(sp, "/") {
			sp += "/"
		}
		if strings.HasPrefix(relKey, sp) {
			if operation == "read" || operation == "find" || operation == "search" {
				return accessResult{allowed: true}
			}
			return accessResult{
				allowed: false,
				reason:  fmt.Sprintf("%s is in %s (read-only) — you can read but not write to shared documents", key, shared),
			}
		}
	}

	return accessResult{
		allowed: false,
		reason:  fmt.Sprintf("%s is outside your prefix (%s) — you can only access files in %s/ and shared prefixes %v", key, cfg.UserPrefix, cfg.UserPrefix, cfg.SharedPrefixes),
	}
}
