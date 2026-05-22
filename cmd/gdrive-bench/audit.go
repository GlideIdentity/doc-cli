package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type auditEntry struct {
	Timestamp string `json:"timestamp"`
	AgentID   string `json:"agent_id"`
	Machine   string `json:"machine"`
	Operation string `json:"operation"`
	FileID    string `json:"file_id,omitempty"`
	FileName  string `json:"file_name,omitempty"`
	Source    string `json:"source,omitempty"`
	Result    string `json:"result"`
	Detail    string `json:"detail,omitempty"`
	LatencyMs int64  `json:"latency_ms"`
}

type auditLogger struct {
	mu       sync.Mutex
	file     *os.File
	agentID  string
	machine  string
}

func newAuditLogger() (*auditLogger, error) {
	logDir := configDir()
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, err
	}

	logPath := filepath.Join(logDir, "audit.jsonl")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}

	hostname, _ := os.Hostname()

	agentID := os.Getenv("GDRIVE_AGENT_ID")
	if agentID == "" {
		agentID = fmt.Sprintf("agent-%d", os.Getpid())
	}

	return &auditLogger{
		file:    f,
		agentID: agentID,
		machine: hostname,
	}, nil
}

func (a *auditLogger) log(op, fileID, fileName, source, result, detail string, latencyMs int64) {
	if a == nil {
		return
	}

	entry := auditEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		AgentID:   a.agentID,
		Machine:   a.machine,
		Operation: op,
		FileID:    fileID,
		FileName:  fileName,
		Source:    source,
		Result:    result,
		Detail:    detail,
		LatencyMs: latencyMs,
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	data, _ := json.Marshal(entry)
	a.file.Write(data)
	a.file.Write([]byte("\n"))
}

func (a *auditLogger) close() {
	if a != nil && a.file != nil {
		a.file.Close()
	}
}
