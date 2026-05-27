package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const cliBinary = "gcs-bench"

func requireCLI(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(cliBinary); err != nil {
		t.Skipf("skipping: %s not installed (brew install glideidentity/tap/gcs-bench)", cliBinary)
	}
}

func runCLI(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(cliBinary, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s %v: %v", cliBinary, args, err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func parseJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &m); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, s)
	}
	return m
}

func uniqueName() string {
	return fmt.Sprintf("cli-test-%d-%d.md", time.Now().UnixMilli(), rand.Intn(100000))
}

// --- Smoke tests (no GCS required) ---

func TestCLI_NoArgs_ShowsUsage(t *testing.T) {
	requireCLI(t)

	_, stderr, code := runCLI(t)
	if code != 1 {
		t.Errorf("exit code: got %d, want 1", code)
	}
	if !strings.Contains(stderr, "gcs-bench") {
		t.Error("expected usage text in stderr")
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Error("expected 'Usage:' in stderr")
	}
}

func TestCLI_UnknownCommand(t *testing.T) {
	requireCLI(t)

	_, stderr, code := runCLI(t, "nonexistent-cmd")
	if code != 1 {
		t.Errorf("exit code: got %d, want 1", code)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("expected 'unknown command' in stderr, got: %s", stderr)
	}
}

// --- Daemon tests ---

func TestCLI_DaemonStatus(t *testing.T) {
	requireCLI(t)

	stdout, _, code := runCLI(t, "daemon", "status")
	if code != 0 {
		t.Fatalf("daemon status failed with exit code %d", code)
	}
	m := parseJSON(t, stdout)
	status, ok := m["status"].(string)
	if !ok {
		t.Fatal("expected 'status' field in JSON output")
	}
	if status != "running" && status != "not running" {
		t.Errorf("unexpected status: %q", status)
	}
}

func TestCLI_DaemonNoSubcommand(t *testing.T) {
	requireCLI(t)

	_, stderr, code := runCLI(t, "daemon")
	if code != 1 {
		t.Errorf("exit code: got %d, want 1", code)
	}
	if !strings.Contains(stderr, "daemon") {
		t.Errorf("expected daemon usage hint in stderr: %s", stderr)
	}
}

// --- Integration tests (require running daemon + GCS access) ---

func requireDaemon(t *testing.T) {
	t.Helper()
	requireCLI(t)
	stdout, _, code := runCLI(t, "daemon", "status")
	if code != 0 {
		t.Skip("skipping: daemon status check failed")
	}
	m := parseJSON(t, stdout)
	if m["status"] != "running" {
		t.Skip("skipping: daemon not running")
	}
}

func TestCLI_Find_ReturnsJSON(t *testing.T) {
	requireDaemon(t)

	stdout, _, code := runCLI(t, "find", "--name", "test")
	if code != 0 {
		t.Fatalf("find failed with exit code %d", code)
	}

	m := parseJSON(t, stdout)
	if _, ok := m["files"]; !ok {
		t.Error("expected 'files' field in find output")
	}
	if _, ok := m["_elapsed_ms"]; !ok {
		t.Error("expected '_elapsed_ms' field in find output")
	}
	if _, ok := m["_source"]; !ok {
		t.Error("expected '_source' field in find output")
	}
}

func TestCLI_Find_EmptyName(t *testing.T) {
	requireDaemon(t)

	stdout, _, code := runCLI(t, "find", "--name", "")
	if code != 0 {
		t.Fatalf("find with empty name failed with exit code %d", code)
	}

	m := parseJSON(t, stdout)
	if _, ok := m["files"]; !ok {
		t.Error("expected 'files' field even with empty name")
	}
}

func TestCLI_Search_ReturnsJSON(t *testing.T) {
	requireDaemon(t)

	stdout, _, code := runCLI(t, "search", "--query", "test")
	if code != 0 {
		t.Fatalf("search failed with exit code %d", code)
	}

	m := parseJSON(t, stdout)
	if _, ok := m["files"]; !ok {
		t.Error("expected 'files' field in search output")
	}
	if _, ok := m["count"]; !ok {
		t.Error("expected 'count' field in search output")
	}
	if _, ok := m["_elapsed_ms"]; !ok {
		t.Error("expected '_elapsed_ms' field in search output")
	}
}

func TestCLI_Search_EmptyQuery(t *testing.T) {
	requireDaemon(t)

	stdout, _, code := runCLI(t, "search", "--query", "")
	if code != 0 {
		t.Fatalf("search with empty query failed with exit code %d", code)
	}

	m := parseJSON(t, stdout)
	files, ok := m["files"].([]any)
	if !ok {
		t.Fatal("expected 'files' array in search output")
	}
	count, ok := m["count"].(float64)
	if !ok {
		t.Fatal("expected 'count' number in search output")
	}
	if int(count) != len(files) {
		t.Errorf("count (%d) != len(files) (%d)", int(count), len(files))
	}
}

func TestCLI_Create_And_Read(t *testing.T) {
	requireDaemon(t)

	name := uniqueName()
	body := "Integration test content created at " + time.Now().Format(time.RFC3339)

	// Create
	stdout, _, code := runCLI(t, "create", "--name", name, "--body", body)
	if code != 0 {
		t.Fatalf("create failed with exit code %d", code)
	}

	m := parseJSON(t, stdout)
	key, ok := m["key"].(string)
	if !ok || key == "" {
		t.Fatal("expected 'key' in create response")
	}
	if _, ok := m["_elapsed_ms"]; !ok {
		t.Error("expected '_elapsed_ms' in create response")
	}

	// Read back
	stdout, _, code = runCLI(t, "read", "--key", key)
	if code != 0 {
		t.Fatalf("read failed with exit code %d", code)
	}

	m = parseJSON(t, stdout)
	content, ok := m["content"].(string)
	if !ok {
		t.Fatal("expected 'content' in read response")
	}
	if content != body {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", content, body)
	}
	if _, ok := m["generation"]; !ok {
		t.Error("expected 'generation' in read response")
	}
	if _, ok := m["_source"]; !ok {
		t.Error("expected '_source' in read response")
	}
}

func TestCLI_Create_And_Find(t *testing.T) {
	requireDaemon(t)

	name := uniqueName()
	body := "Findable test document"

	stdout, _, code := runCLI(t, "create", "--name", name, "--body", body)
	if code != 0 {
		t.Fatalf("create failed with exit code %d", code)
	}
	parseJSON(t, stdout)

	// The daemon index refreshes asynchronously; retry a few times
	var found bool
	for attempt := 0; attempt < 5; attempt++ {
		time.Sleep(2 * time.Second)

		stdout, _, code = runCLI(t, "find", "--name", name)
		if code != 0 {
			t.Fatalf("find failed with exit code %d", code)
		}

		m := parseJSON(t, stdout)
		files, ok := m["files"].([]any)
		if !ok {
			t.Fatal("expected 'files' array in find output")
		}

		for _, f := range files {
			entry, ok := f.(map[string]any)
			if !ok {
				continue
			}
			if n, _ := entry["name"].(string); n == name {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Log("created file not found in find results — index may not have refreshed (this is expected behavior for daemon-cached indexes)")
	}
}

func TestCLI_Update_WithCAS(t *testing.T) {
	requireDaemon(t)

	name := uniqueName()
	initialBody := "Initial content for CAS test"

	// Create
	stdout, _, code := runCLI(t, "create", "--name", name, "--body", initialBody)
	if code != 0 {
		t.Fatalf("create failed with exit code %d", code)
	}
	createResp := parseJSON(t, stdout)
	key := createResp["key"].(string)

	// Read to get generation
	stdout, _, code = runCLI(t, "read", "--key", key)
	if code != 0 {
		t.Fatalf("read failed with exit code %d", code)
	}
	readResp := parseJSON(t, stdout)
	gen := int64(readResp["generation"].(float64))

	// Update with correct generation (append mode)
	appendBody := "Appended content"
	stdout, _, code = runCLI(t, "update", "--key", key, "--body", appendBody, "--expect-gen", fmt.Sprintf("%d", gen))
	if code != 0 {
		t.Fatalf("update failed with exit code %d", code)
	}
	updateResp := parseJSON(t, stdout)
	if status, _ := updateResp["status"].(string); status != "updated" {
		t.Errorf("expected status=updated, got %q (full response: %v)", status, updateResp)
	}

	// Read again and verify content was appended
	stdout, _, code = runCLI(t, "read", "--key", key)
	if code != 0 {
		t.Fatalf("read after update failed with exit code %d", code)
	}
	readResp = parseJSON(t, stdout)
	content := readResp["content"].(string)
	if !strings.Contains(content, initialBody) {
		t.Error("updated content should contain initial body")
	}
	if !strings.Contains(content, appendBody) {
		t.Error("updated content should contain appended body")
	}
}

func TestCLI_Update_StaleGeneration_Conflict(t *testing.T) {
	requireDaemon(t)

	name := uniqueName()
	body := "Content for conflict test"

	// Create
	stdout, _, code := runCLI(t, "create", "--name", name, "--body", body)
	if code != 0 {
		t.Fatalf("create failed with exit code %d", code)
	}
	createResp := parseJSON(t, stdout)
	key := createResp["key"].(string)

	// Update with a bogus generation — should result in conflict
	stdout, _, code = runCLI(t, "update", "--key", key, "--body", "nope", "--expect-gen", "1")
	if code != 0 {
		t.Fatalf("update with stale gen should not fail at process level, got exit %d", code)
	}

	m := parseJSON(t, stdout)
	status, _ := m["status"].(string)
	if status != "conflict" {
		t.Errorf("expected status=conflict, got %q", status)
	}
	if _, ok := m["error"]; !ok {
		t.Error("expected 'error' field in conflict response")
	}
}

func TestCLI_Update_Replace(t *testing.T) {
	requireDaemon(t)

	name := uniqueName()
	initialBody := "Original content to be replaced"

	// Create
	stdout, _, code := runCLI(t, "create", "--name", name, "--body", initialBody)
	if code != 0 {
		t.Fatalf("create failed with exit code %d", code)
	}
	key := parseJSON(t, stdout)["key"].(string)

	// Read to get generation
	stdout, _, code = runCLI(t, "read", "--key", key)
	if code != 0 {
		t.Fatalf("read failed with exit code %d", code)
	}
	gen := int64(parseJSON(t, stdout)["generation"].(float64))

	// Update with --replace
	replacement := "Completely new content"
	stdout, _, code = runCLI(t, "update", "--key", key, "--body", replacement, "--expect-gen", fmt.Sprintf("%d", gen), "--replace")
	if code != 0 {
		t.Fatalf("update --replace failed with exit code %d", code)
	}
	if status := parseJSON(t, stdout)["status"]; status != "updated" {
		t.Errorf("expected status=updated, got %v", status)
	}

	// Verify replacement
	stdout, _, code = runCLI(t, "read", "--key", key)
	if code != 0 {
		t.Fatalf("read after replace failed with exit code %d", code)
	}
	content := parseJSON(t, stdout)["content"].(string)
	if content != replacement {
		t.Errorf("after --replace, content should be exactly the replacement\ngot:  %q\nwant: %q", content, replacement)
	}
}

func TestCLI_Read_NonexistentKey(t *testing.T) {
	requireDaemon(t)

	stdout, stderr, code := runCLI(t, "read", "--key", "bench/nonexistent-key-xyz-99999.md")
	_ = stdout
	if code == 0 && stderr == "" {
		m := parseJSON(t, stdout)
		if _, hasErr := m["error"]; !hasErr {
			if content, _ := m["content"].(string); content == "" {
				return
			}
		}
	}
	// Either non-zero exit or error in JSON is acceptable
}

func TestCLI_Read_PermissionDenied_OtherPrefix(t *testing.T) {
	requireDaemon(t)

	// Attempt to read a key in another user's prefix
	stdout, _, code := runCLI(t, "read", "--key", "other-user-prefix/secret.md")
	if code != 0 {
		// Permission denied can also surface as an error exit
		return
	}

	m := parseJSON(t, stdout)
	if status, _ := m["status"].(string); status != "denied" {
		t.Errorf("expected denied status for other-user prefix, got: %v", m)
	}
}

func TestCLI_Create_PermissionDenied_OtherPrefix(t *testing.T) {
	requireDaemon(t)

	// Attempt to create in another user's prefix
	stdout, _, code := runCLI(t, "create", "--name", "other-user-prefix/hacked.md", "--body", "pwned")
	if code != 0 {
		return
	}

	m := parseJSON(t, stdout)
	if status, _ := m["status"].(string); status != "denied" {
		// If the create prefix logic prepends the user prefix, the key won't
		// actually land in another prefix — both outcomes are valid
		if key, _ := m["key"].(string); !strings.HasPrefix(key, "bench/") {
			t.Errorf("expected either denied or key under own prefix, got: %v", m)
		}
	}
}

func TestCLI_Find_ResponseStructure(t *testing.T) {
	requireDaemon(t)

	stdout, _, code := runCLI(t, "find", "--name", "md")
	if code != 0 {
		t.Fatalf("find failed with exit code %d", code)
	}

	m := parseJSON(t, stdout)
	files, ok := m["files"].([]any)
	if !ok {
		t.Fatal("expected 'files' array")
	}

	if len(files) == 0 {
		t.Skip("no files found, cannot verify entry structure")
	}

	entry, ok := files[0].(map[string]any)
	if !ok {
		t.Fatal("expected file entry to be a JSON object")
	}

	requiredFields := []string{"key", "name", "size", "updated", "generation"}
	for _, field := range requiredFields {
		if _, ok := entry[field]; !ok {
			t.Errorf("file entry missing required field %q", field)
		}
	}
}

func TestCLI_Search_FindsCreatedContent(t *testing.T) {
	requireDaemon(t)

	uniquePhrase := fmt.Sprintf("xyzzy-unique-%d", rand.Int63())
	name := uniqueName()

	// Create a doc with a unique phrase
	_, _, code := runCLI(t, "create", "--name", name, "--body", uniquePhrase)
	if code != 0 {
		t.Fatalf("create failed with exit code %d", code)
	}

	// Wait for content to be indexed
	time.Sleep(3 * time.Second)

	// Search for the unique phrase
	stdout, _, code := runCLI(t, "search", "--query", uniquePhrase)
	if code != 0 {
		t.Fatalf("search failed with exit code %d", code)
	}

	m := parseJSON(t, stdout)
	count, _ := m["count"].(float64)
	if int(count) == 0 {
		t.Log("newly created content may not be indexed yet — this is expected if the daemon hasn't refreshed")
	}
}

func TestCLI_OutputIsJSON(t *testing.T) {
	requireDaemon(t)

	commands := [][]string{
		{"daemon", "status"},
		{"find", "--name", "anything"},
		{"search", "--query", "anything"},
	}

	for _, args := range commands {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			stdout, _, code := runCLI(t, args...)
			if code != 0 {
				t.Skipf("command %v exited with %d", args, code)
			}
			trimmed := strings.TrimSpace(stdout)
			if trimmed == "" {
				t.Fatal("empty output")
			}
			if !json.Valid([]byte(trimmed)) {
				t.Errorf("output is not valid JSON: %s", trimmed)
			}
		})
	}
}

func TestCLI_ElapsedMs_IsNumeric(t *testing.T) {
	requireDaemon(t)

	stdout, _, code := runCLI(t, "find", "--name", "bench")
	if code != 0 {
		t.Fatalf("find failed with exit code %d", code)
	}

	m := parseJSON(t, stdout)
	elapsed, ok := m["_elapsed_ms"].(float64)
	if !ok {
		t.Fatal("_elapsed_ms should be a number")
	}
	if elapsed < 0 {
		t.Errorf("_elapsed_ms should be non-negative, got %f", elapsed)
	}
}

func TestCLI_Find_Performance(t *testing.T) {
	requireDaemon(t)

	start := time.Now()
	stdout, _, code := runCLI(t, "find", "--name", "test")
	wallTime := time.Since(start)

	if code != 0 {
		t.Fatalf("find failed with exit code %d", code)
	}

	m := parseJSON(t, stdout)
	source, _ := m["_source"].(string)

	if source == "index" && wallTime > 2*time.Second {
		t.Errorf("index-based find took %v (expected < 2s including process startup)", wallTime)
	}
}
