package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/api/drive/v3"
)

func runCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	name := fs.String("name", "", "document name")
	body := fs.String("body", "", "document content")
	folder := fs.String("folder", folderID(), "parent folder ID")
	fs.Parse(args)

	if *name == "" {
		return fmt.Errorf("--name is required")
	}

	start := time.Now()

	svc, err := driveService()
	if err != nil {
		return err
	}

	meta := &drive.File{
		Name:     *name,
		MimeType: "application/vnd.google-apps.document",
	}
	if *folder != "" {
		meta.Parents = []string{*folder}
	}

	created, err := svc.Files.Create(meta).
		Fields("id, name, mimeType, modifiedTime").
		Do()
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	// If body content provided, write it to the doc
	if *body != "" {
		_, err = svc.Files.Update(created.Id, nil).
			Media(strings.NewReader(*body)).
			Do()
		if err != nil {
			return fmt.Errorf("write content: %w", err)
		}
	}

	out := struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		ElapsedMs int64  `json:"_elapsed_ms"`
	}{
		ID:        created.Id,
		Name:      created.Name,
		ElapsedMs: time.Since(start).Milliseconds(),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "")
	return enc.Encode(out)
}
