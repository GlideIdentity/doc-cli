package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func runRead(args []string) error {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	id := fs.String("id", "", "file ID to read")
	mime := fs.String("mime", "", "skip metadata lookup — treat as this MIME type (e.g. 'google-doc')")
	fs.Parse(args)

	if *id == "" {
		return fmt.Errorf("--id is required")
	}

	start := time.Now()

	svc, err := driveService()
	if err != nil {
		return err
	}

	var content string
	var name string

	isGoogleApp := false
	if *mime != "" {
		// Caller told us the type — skip the metadata API call entirely
		isGoogleApp = *mime == "google-doc" || *mime == "google-sheet" || *mime == "google-slides" ||
			strings.HasPrefix(*mime, "application/vnd.google-apps.")
		name = *id
	} else {
		file, err := svc.Files.Get(*id).Fields("mimeType, name").Do()
		if err != nil {
			return fmt.Errorf("get file metadata: %w", err)
		}
		name = file.Name
		isGoogleApp = strings.HasPrefix(file.MimeType, "application/vnd.google-apps.")
	}

	if isGoogleApp {
		resp, err := svc.Files.Export(*id, "text/plain").Download()
		if err != nil {
			return fmt.Errorf("export: %w", err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read export body: %w", err)
		}
		content = string(b)
	} else {
		resp, err := svc.Files.Get(*id).Download()
		if err != nil {
			return fmt.Errorf("download: %w", err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read body: %w", err)
		}
		content = string(b)
	}

	out := struct {
		Name      string `json:"name"`
		Content   string `json:"content"`
		ElapsedMs int64  `json:"_elapsed_ms"`
	}{
		Name:      name,
		Content:   content,
		ElapsedMs: time.Since(start).Milliseconds(),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "")
	return enc.Encode(out)
}
