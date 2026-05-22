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

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	id := fs.String("id", "", "file ID to update")
	body := fs.String("body", "", "content to append")
	replace := fs.Bool("replace", false, "replace content instead of appending")
	expect := fs.String("expect-modified", "", "fail if file's modifiedTime differs (optimistic concurrency)")
	fs.Parse(args)

	if *id == "" {
		return fmt.Errorf("--id is required")
	}
	if *body == "" {
		return fmt.Errorf("--body is required")
	}

	start := time.Now()

	svc, err := driveService()
	if err != nil {
		return err
	}

	// Get current file state
	file, err := svc.Files.Get(*id).Fields("mimeType, modifiedTime, name").Do()
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}

	// Optimistic concurrency check
	if *expect != "" && file.ModifiedTime != *expect {
		out := struct {
			ID           string `json:"id"`
			Status       string `json:"status"`
			Error        string `json:"error"`
			Expected     string `json:"expected_modified"`
			Actual       string `json:"actual_modified"`
			ElapsedMs    int64  `json:"_elapsed_ms"`
		}{
			ID:           *id,
			Status:       "conflict",
			Error:        "file was modified by another user since you last read it",
			Expected:     *expect,
			Actual:       file.ModifiedTime,
			ElapsedMs:    time.Since(start).Milliseconds(),
		}
		enc := json.NewEncoder(os.Stdout)
		return enc.Encode(out)
	}

	var newContent string
	if *replace {
		newContent = *body
	} else {
		var existing string
		if strings.HasPrefix(file.MimeType, "application/vnd.google-apps.") {
			resp, err := svc.Files.Export(*id, "text/plain").Download()
			if err != nil {
				return fmt.Errorf("export existing: %w", err)
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			existing = string(b)
		}
		newContent = existing + "\n\n" + *body
	}

	_, err = svc.Files.Update(*id, nil).
		Media(strings.NewReader(newContent)).
		Do()
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}

	out := struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Status    string `json:"status"`
		ElapsedMs int64  `json:"_elapsed_ms"`
	}{
		ID:        *id,
		Name:      file.Name,
		Status:    "updated",
		ElapsedMs: time.Since(start).Milliseconds(),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "")
	return enc.Encode(out)
}
