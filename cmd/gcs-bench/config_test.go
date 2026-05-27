package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func resetConfigCache() {
	cachedConfig = nil
}

// --- validateAccess ---

func TestValidateAccess_OwnPrefixAllowed(t *testing.T) {
	cachedConfig = &appConfig{
		Bucket:         "test-bucket",
		UserPrefix:     "team-alpha",
		SharedPrefixes: []string{"shared"},
	}
	defer resetConfigCache()

	for _, op := range []string{"read", "find", "search", "create", "update"} {
		result := validateAccess("team-alpha/notes.md", op)
		if !result.allowed {
			t.Errorf("op=%s: own prefix should be allowed, got denied: %s", op, result.reason)
		}
	}
}

func TestValidateAccess_OwnPrefixWithTrailingSlash(t *testing.T) {
	cachedConfig = &appConfig{
		Bucket:         "test-bucket",
		UserPrefix:     "team-alpha/",
		SharedPrefixes: []string{"shared"},
	}
	defer resetConfigCache()

	result := validateAccess("team-alpha/deep/path/file.md", "create")
	if !result.allowed {
		t.Errorf("own prefix with trailing slash should be allowed: %s", result.reason)
	}
}

func TestValidateAccess_OtherPrefixDenied(t *testing.T) {
	cachedConfig = &appConfig{
		Bucket:         "test-bucket",
		UserPrefix:     "team-alpha",
		SharedPrefixes: []string{"shared"},
	}
	defer resetConfigCache()

	for _, op := range []string{"read", "create", "update"} {
		result := validateAccess("team-beta/secrets.md", op)
		if result.allowed {
			t.Errorf("op=%s: other prefix should be denied", op)
		}
		if result.reason == "" {
			t.Errorf("op=%s: denial should include a reason", op)
		}
	}
}

func TestValidateAccess_SharedReadOnly(t *testing.T) {
	cachedConfig = &appConfig{
		Bucket:         "test-bucket",
		UserPrefix:     "team-alpha",
		SharedPrefixes: []string{"shared", "public-docs"},
	}
	defer resetConfigCache()

	readOps := []string{"read", "find", "search"}
	for _, op := range readOps {
		result := validateAccess("shared/guide.md", op)
		if !result.allowed {
			t.Errorf("op=%s: shared prefix read should be allowed: %s", op, result.reason)
		}
	}

	for _, op := range readOps {
		result := validateAccess("public-docs/readme.md", op)
		if !result.allowed {
			t.Errorf("op=%s: second shared prefix read should be allowed: %s", op, result.reason)
		}
	}
}

func TestValidateAccess_SharedWriteDenied(t *testing.T) {
	cachedConfig = &appConfig{
		Bucket:         "test-bucket",
		UserPrefix:     "team-alpha",
		SharedPrefixes: []string{"shared"},
	}
	defer resetConfigCache()

	for _, op := range []string{"create", "update"} {
		result := validateAccess("shared/guide.md", op)
		if result.allowed {
			t.Errorf("op=%s: shared prefix write should be denied", op)
		}
		if result.reason == "" {
			t.Errorf("op=%s: denial should include a reason", op)
		}
	}
}

func TestValidateAccess_NoPrefixAllowsAll(t *testing.T) {
	cachedConfig = &appConfig{
		Bucket:     "test-bucket",
		UserPrefix: "",
	}
	defer resetConfigCache()

	for _, op := range []string{"read", "create", "update", "find", "search"} {
		result := validateAccess("any/path/file.md", op)
		if !result.allowed {
			t.Errorf("op=%s: no prefix configured should allow all: %s", op, result.reason)
		}
	}
}

func TestValidateAccess_SharedPrefixWithoutTrailingSlash(t *testing.T) {
	cachedConfig = &appConfig{
		Bucket:         "test-bucket",
		UserPrefix:     "team-alpha",
		SharedPrefixes: []string{"shared"},
	}
	defer resetConfigCache()

	result := validateAccess("shared/nested/doc.md", "read")
	if !result.allowed {
		t.Errorf("shared prefix without trailing slash should still match: %s", result.reason)
	}
}

func TestValidateAccess_KeyOutsideAllPrefixes(t *testing.T) {
	cachedConfig = &appConfig{
		Bucket:         "test-bucket",
		UserPrefix:     "team-alpha",
		SharedPrefixes: []string{"shared"},
	}
	defer resetConfigCache()

	result := validateAccess("unknown-prefix/file.md", "read")
	if result.allowed {
		t.Error("key outside all prefixes should be denied")
	}
	if result.reason == "" {
		t.Error("denial should include a reason mentioning the user prefix")
	}
}

// --- loadConfig ---

func TestLoadConfig_CachedReturnsSamePointer(t *testing.T) {
	orig := &appConfig{Bucket: "cached-bucket", AgentID: "test-agent"}
	cachedConfig = orig
	defer resetConfigCache()

	got := loadConfig()
	if got != orig {
		t.Error("loadConfig should return the cached config pointer")
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	resetConfigCache()
	defer resetConfigCache()

	t.Setenv("GCS_BUCKET", "env-bucket")
	t.Setenv("GCS_PREFIX", "env-prefix")
	t.Setenv("GCS_AGENT_ID", "env-agent")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/tmp/fake-creds.json")

	cfg := loadConfig()
	if cfg.Bucket != "env-bucket" {
		t.Errorf("Bucket: got %q, want %q", cfg.Bucket, "env-bucket")
	}
	if cfg.UserPrefix != "env-prefix" {
		t.Errorf("UserPrefix: got %q, want %q", cfg.UserPrefix, "env-prefix")
	}
	if cfg.AgentID != "env-agent" {
		t.Errorf("AgentID: got %q, want %q", cfg.AgentID, "env-agent")
	}
	if cfg.Credentials != "/tmp/fake-creds.json" {
		t.Errorf("Credentials: got %q, want %q", cfg.Credentials, "/tmp/fake-creds.json")
	}
}

func TestLoadConfig_DefaultAgentID(t *testing.T) {
	resetConfigCache()
	defer resetConfigCache()

	t.Setenv("GCS_AGENT_ID", "")
	t.Setenv("GCS_BUCKET", "")
	t.Setenv("GCS_PREFIX", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")

	cfg := loadConfig()
	if cfg.AgentID == "" {
		t.Error("AgentID should default to user@hostname when not set")
	}
}

// --- prefix / objectKey ---

func TestPrefix_AddsTrailingSlash(t *testing.T) {
	cachedConfig = &appConfig{UserPrefix: "myprefix"}
	defer resetConfigCache()

	p := prefix()
	if p != "myprefix/" {
		t.Errorf("prefix(): got %q, want %q", p, "myprefix/")
	}
}

func TestPrefix_PreservesExistingSlash(t *testing.T) {
	cachedConfig = &appConfig{UserPrefix: "myprefix/"}
	defer resetConfigCache()

	p := prefix()
	if p != "myprefix/" {
		t.Errorf("prefix(): got %q, want %q", p, "myprefix/")
	}
}

func TestPrefix_EmptyReturnsEmpty(t *testing.T) {
	cachedConfig = &appConfig{UserPrefix: ""}
	defer resetConfigCache()

	p := prefix()
	if p != "" {
		t.Errorf("prefix(): got %q, want %q", p, "")
	}
}

func TestObjectKey(t *testing.T) {
	cachedConfig = &appConfig{UserPrefix: "team-alpha"}
	defer resetConfigCache()

	key := objectKey("notes.md")
	if key != "team-alpha/notes.md" {
		t.Errorf("objectKey(): got %q, want %q", key, "team-alpha/notes.md")
	}
}

// --- saveConfig roundtrip ---

func TestSaveConfig_RoundTrip(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	resetConfigCache()
	defer resetConfigCache()

	want := &appConfig{
		Bucket:         "roundtrip-bucket",
		UserPrefix:     "rt-prefix",
		SharedPrefixes: []string{"shared", "docs"},
		AgentID:        "rt-agent",
		Credentials:    "/path/to/creds.json",
	}

	if err := saveConfig(want); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	cfgPath := filepath.Join(tmpHome, ".config", "gcs-bench", "config.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}

	var got appConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal saved config: %v", err)
	}

	if got.Bucket != want.Bucket {
		t.Errorf("Bucket: got %q, want %q", got.Bucket, want.Bucket)
	}
	if got.UserPrefix != want.UserPrefix {
		t.Errorf("UserPrefix: got %q, want %q", got.UserPrefix, want.UserPrefix)
	}
	if got.AgentID != want.AgentID {
		t.Errorf("AgentID: got %q, want %q", got.AgentID, want.AgentID)
	}
	if len(got.SharedPrefixes) != 2 || got.SharedPrefixes[0] != "shared" || got.SharedPrefixes[1] != "docs" {
		t.Errorf("SharedPrefixes: got %v, want %v", got.SharedPrefixes, want.SharedPrefixes)
	}
}

func TestSaveConfig_CreatesDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	resetConfigCache()
	defer resetConfigCache()

	cfg := &appConfig{Bucket: "test"}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	dir := filepath.Join(tmpHome, ".config", "gcs-bench")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("config dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("config dir path is not a directory")
	}
}

func TestSaveConfig_FilePermissions(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	resetConfigCache()
	defer resetConfigCache()

	cfg := &appConfig{Bucket: "perm-test"}
	if err := saveConfig(cfg); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	cfgPath := filepath.Join(tmpHome, ".config", "gcs-bench", "config.json")
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions: got %o, want 0600", perm)
	}
}

func TestLoadConfig_FromFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("GCS_BUCKET", "")
	t.Setenv("GCS_PREFIX", "")
	t.Setenv("GCS_AGENT_ID", "file-agent")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	resetConfigCache()
	defer resetConfigCache()

	cfgDir := filepath.Join(tmpHome, ".config", "gcs-bench")
	os.MkdirAll(cfgDir, 0700)

	fileCfg := &appConfig{
		Bucket:         "file-bucket",
		UserPrefix:     "file-prefix",
		SharedPrefixes: []string{"shared"},
		AgentID:        "file-agent-id",
	}
	data, _ := json.Marshal(fileCfg)
	os.WriteFile(filepath.Join(cfgDir, "config.json"), data, 0600)

	cfg := loadConfig()
	if cfg.Bucket != "file-bucket" {
		t.Errorf("Bucket: got %q, want %q", cfg.Bucket, "file-bucket")
	}
	if cfg.UserPrefix != "file-prefix" {
		t.Errorf("UserPrefix: got %q, want %q", cfg.UserPrefix, "file-prefix")
	}
}

func TestLoadConfig_EnvOverridesFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	resetConfigCache()
	defer resetConfigCache()

	cfgDir := filepath.Join(tmpHome, ".config", "gcs-bench")
	os.MkdirAll(cfgDir, 0700)

	fileCfg := &appConfig{Bucket: "file-bucket", UserPrefix: "file-prefix"}
	data, _ := json.Marshal(fileCfg)
	os.WriteFile(filepath.Join(cfgDir, "config.json"), data, 0600)

	t.Setenv("GCS_BUCKET", "env-overridden")
	t.Setenv("GCS_PREFIX", "")
	t.Setenv("GCS_AGENT_ID", "env-agent-2")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")

	cfg := loadConfig()
	if cfg.Bucket != "env-overridden" {
		t.Errorf("Bucket should be env-overridden, got %q", cfg.Bucket)
	}
	if cfg.UserPrefix != "file-prefix" {
		t.Errorf("UserPrefix should come from file when env is empty, got %q", cfg.UserPrefix)
	}
}
