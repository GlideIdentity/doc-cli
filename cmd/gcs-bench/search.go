package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

func runSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	query := fs.String("query", "", "full-text search query")
	fs.Parse(args)

	if *query == "" {
		return fmt.Errorf("--query is required")
	}

	start := time.Now()
	ctx := context.Background()

	client, err := gcsClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	queryLower := strings.ToLower(*query)
	var files []objectEntry

	it := bucket(client).Objects(ctx, &storage.Query{Prefix: prefix()})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("list: %w", err)
		}

		reader, err := bucket(client).Object(attrs.Name).NewReader(ctx)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(reader)
		reader.Close()

		if strings.Contains(strings.ToLower(string(b)), queryLower) {
			files = append(files, objectEntry{
				Key:        attrs.Name,
				Name:       strings.TrimPrefix(attrs.Name, prefix()),
				Size:       attrs.Size,
				Updated:    attrs.Updated.UTC().Format(time.RFC3339),
				Generation: attrs.Generation,
			})
		}
	}

	out := struct {
		Files     []objectEntry `json:"files"`
		Count     int           `json:"count"`
		ElapsedMs int64         `json:"_elapsed_ms"`
	}{Files: files, Count: len(files), ElapsedMs: time.Since(start).Milliseconds()}

	if out.Files == nil {
		out.Files = []objectEntry{}
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
