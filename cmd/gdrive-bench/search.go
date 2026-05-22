package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

func runSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	query := fs.String("query", "", "full-text search query")
	folder := fs.String("folder", folderID(), "folder ID to scope search")
	fs.Parse(args)

	if *query == "" {
		return fmt.Errorf("--query is required")
	}

	start := time.Now()

	svc, err := driveService()
	if err != nil {
		return err
	}

	q := fmt.Sprintf("fullText contains '%s' and trashed = false", *query)
	if *folder != "" {
		q += fmt.Sprintf(" and '%s' in parents", *folder)
	}

	list, err := svc.Files.List().
		Q(q).
		Fields("files(id, name, mimeType, modifiedTime)").
		PageSize(50).
		Do()
	if err != nil {
		return fmt.Errorf("drive search: %w", err)
	}

	type fileEntry struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		MimeType     string `json:"mimeType"`
		ModifiedTime string `json:"modifiedTime"`
	}

	files := make([]fileEntry, len(list.Files))
	for i, f := range list.Files {
		files[i] = fileEntry{ID: f.Id, Name: f.Name, MimeType: f.MimeType, ModifiedTime: f.ModifiedTime}
	}

	out := struct {
		Files     []fileEntry `json:"files"`
		Count     int         `json:"count"`
		ElapsedMs int64       `json:"_elapsed_ms"`
	}{
		Files:     files,
		Count:     len(files),
		ElapsedMs: time.Since(start).Milliseconds(),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "")
	return enc.Encode(out)
}
