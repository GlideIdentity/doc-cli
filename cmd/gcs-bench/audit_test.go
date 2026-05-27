package main

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

func TestAuditLogger_WritesJSONL(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "audit-*.jsonl")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	logger := &auditLogger{
		file:       f,
		agentID:    "test-agent",
		machine:    "test-machine",
		userPrefix: "test-prefix",
	}
	defer logger.close()

	logger.log("read", "prefix/file.md", "file.md", "cache", "ok", "len=42", 5)

	f.Sync()

	readF, err := os.Open(f.Name())
	if err != nil {
		t.Fatalf("reopen file: %v", err)
	}
	defer readF.Close()

	scanner := bufio.NewScanner(readF)
	if !scanner.Scan() {
		t.Fatal("expected at least one line in audit log")
	}

	var entry auditEntry
	if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
		t.Fatalf("unmarshal audit entry: %v", err)
	}

	if entry.AgentID != "test-agent" {
		t.Errorf("AgentID: got %q, want %q", entry.AgentID, "test-agent")
	}
	if entry.Machine != "test-machine" {
		t.Errorf("Machine: got %q, want %q", entry.Machine, "test-machine")
	}
	if entry.UserPrefix != "test-prefix" {
		t.Errorf("UserPrefix: got %q, want %q", entry.UserPrefix, "test-prefix")
	}
	if entry.Operation != "read" {
		t.Errorf("Operation: got %q, want %q", entry.Operation, "read")
	}
	if entry.Key != "prefix/file.md" {
		t.Errorf("Key: got %q, want %q", entry.Key, "prefix/file.md")
	}
	if entry.Name != "file.md" {
		t.Errorf("Name: got %q, want %q", entry.Name, "file.md")
	}
	if entry.Source != "cache" {
		t.Errorf("Source: got %q, want %q", entry.Source, "cache")
	}
	if entry.Result != "ok" {
		t.Errorf("Result: got %q, want %q", entry.Result, "ok")
	}
	if entry.Detail != "len=42" {
		t.Errorf("Detail: got %q, want %q", entry.Detail, "len=42")
	}
	if entry.LatencyMs != 5 {
		t.Errorf("LatencyMs: got %d, want %d", entry.LatencyMs, 5)
	}
	if entry.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
}

func TestAuditLogger_MultipleEntries(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "audit-*.jsonl")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	logger := &auditLogger{
		file:       f,
		agentID:    "agent",
		machine:    "machine",
		userPrefix: "pfx",
	}
	defer logger.close()

	logger.log("create", "pfx/a.md", "a.md", "api", "ok", "", 10)
	logger.log("read", "pfx/b.md", "b.md", "cache", "ok", "len=100", 2)
	logger.log("update", "pfx/c.md", "c.md", "api", "error", "conflict", 15)

	f.Sync()

	readF, err := os.Open(f.Name())
	if err != nil {
		t.Fatalf("reopen file: %v", err)
	}
	defer readF.Close()

	scanner := bufio.NewScanner(readF)
	var entries []auditEntry
	for scanner.Scan() {
		var e auditEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		entries = append(entries, e)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].Operation != "create" {
		t.Errorf("entry 0 Operation: got %q, want %q", entries[0].Operation, "create")
	}
	if entries[1].Operation != "read" {
		t.Errorf("entry 1 Operation: got %q, want %q", entries[1].Operation, "read")
	}
	if entries[2].Operation != "update" {
		t.Errorf("entry 2 Operation: got %q, want %q", entries[2].Operation, "update")
	}
	if entries[2].Result != "error" {
		t.Errorf("entry 2 Result: got %q, want %q", entries[2].Result, "error")
	}
}

func TestAuditLogger_NilLoggerNoOp(t *testing.T) {
	var logger *auditLogger
	logger.log("read", "key", "name", "source", "ok", "detail", 1)
}

func TestAuditLogger_NilClose(t *testing.T) {
	var logger *auditLogger
	logger.close()
}

func TestAuditLogger_EmptyOptionalFields(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "audit-*.jsonl")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	logger := &auditLogger{
		file:    f,
		agentID: "agent",
		machine: "machine",
	}
	defer logger.close()

	logger.log("find", "", "query", "", "ok", "", 3)
	f.Sync()

	readF, err := os.Open(f.Name())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer readF.Close()

	scanner := bufio.NewScanner(readF)
	if !scanner.Scan() {
		t.Fatal("expected a line")
	}

	raw := scanner.Bytes()

	var m map[string]any
	json.Unmarshal(raw, &m)

	if _, ok := m["user_prefix"]; ok {
		t.Error("user_prefix should be omitted when empty (omitempty)")
	}
}

func TestAuditLogger_TimestampFormat(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "audit-*.jsonl")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	logger := &auditLogger{
		file:       f,
		agentID:    "agent",
		machine:    "machine",
		userPrefix: "pfx",
	}
	defer logger.close()

	logger.log("read", "key", "name", "api", "ok", "", 0)
	f.Sync()

	readF, err := os.Open(f.Name())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer readF.Close()

	scanner := bufio.NewScanner(readF)
	scanner.Scan()

	var entry auditEntry
	json.Unmarshal(scanner.Bytes(), &entry)

	if len(entry.Timestamp) < 20 {
		t.Errorf("timestamp looks too short for RFC3339Nano: %q", entry.Timestamp)
	}
}
