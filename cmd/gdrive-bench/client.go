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
	"time"
)

func daemonClient() *http.Client {
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

	resp, err := daemonClient().Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func daemonPost(path string, body any) ([]byte, error) {
	buf, _ := json.Marshal(body)
	resp, err := daemonClient().Post("http://daemon"+path, "application/json", bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
