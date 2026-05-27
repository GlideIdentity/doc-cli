package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"time"
)

func socketPath() string {
	return filepath.Join(configDir(), "daemon.sock")
}

func pidFilePath() string {
	return filepath.Join(configDir(), "daemon.pid")
}

func daemonHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath())
			},
		},
		Timeout: 30 * time.Second,
	}
}

func isDaemonRunning() bool {
	conn, err := net.Dial("unix", socketPath())
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func daemonGet(path string, params map[string]string) ([]byte, error) {
	u := url.URL{Scheme: "http", Host: "daemon", Path: path}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	resp, err := daemonHTTPClient().Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("daemon: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func daemonPost(path string, body any) ([]byte, error) {
	buf, _ := json.Marshal(body)
	resp, err := daemonHTTPClient().Post("http://daemon"+path, "application/json", bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("daemon: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
