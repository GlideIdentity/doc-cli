package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type auditEntry struct {
	Timestamp  string `json:"timestamp"`
	AgentID    string `json:"agent_id"`
	Machine    string `json:"machine"`
	UserPrefix string `json:"user_prefix,omitempty"`
	Operation  string `json:"operation"`
	Key        string `json:"key,omitempty"`
	Name       string `json:"name,omitempty"`
	Source     string `json:"source,omitempty"`
	Result     string `json:"result"`
	Detail     string `json:"detail,omitempty"`
	LatencyMs  int64  `json:"latency_ms"`
}

type auditLogger struct {
	mu         sync.Mutex
	file       *os.File
	agentID    string
	machine    string
	userPrefix string
}

func newAuditLogger() (*auditLogger, error) {
	logDir := configDir()
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(logDir, "audit.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	hostname, _ := os.Hostname()
	cfg := loadConfig()
	return &auditLogger{
		file:       f,
		agentID:    cfg.AgentID,
		machine:    hostname,
		userPrefix: cfg.UserPrefix,
	}, nil
}

func (a *auditLogger) log(op, key, name, source, result, detail string, latencyMs int64) {
	if a == nil {
		return
	}
	entry := auditEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		AgentID:    a.agentID,
		Machine:    a.machine,
		UserPrefix: a.userPrefix,
		Operation:  op,
		Key:        key,
		Name:       name,
		Source:     source,
		Result:     result,
		Detail:     detail,
		LatencyMs:  latencyMs,
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
