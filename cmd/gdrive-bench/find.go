package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

func runFind(args []string) error {
	fs := flag.NewFlagSet("find", flag.ExitOnError)
	name := fs.String("name", "", "file name to search for")
	folder := fs.String("folder", folderID(), "folder ID to scope search")
	fs.Parse(args)

	if *name == "" {
		return fmt.Errorf("--name is required")
	}

	start := time.Now()

	svc, err := driveService()
	if err != nil {
		return err
	}

	q := fmt.Sprintf("name contains '%s' and trashed = false", *name)
	if *folder != "" {
		q += fmt.Sprintf(" and '%s' in parents", *folder)
	}

	list, err := svc.Files.List().
		Q(q).
		Fields("files(id, name, mimeType, modifiedTime)").
		PageSize(50).
		Do()
	if err != nil {
		return fmt.Errorf("drive list: %w", err)
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
		ElapsedMs int64       `json:"_elapsed_ms"`
	}{
		Files:     files,
		ElapsedMs: time.Since(start).Milliseconds(),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "")
	return enc.Encode(out)
}
