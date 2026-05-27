package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type objectEntry struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Updated    string `json:"updated"`
	Generation int64  `json:"generation"`
}

func runFind(args []string) error {
	fs := flag.NewFlagSet("find", flag.ExitOnError)
	name := fs.String("name", "", "object name to search for")
	fs.Parse(args)

	if *name == "" {
		return fmt.Errorf("--name is required")
	}

	start := time.Now()
	ctx := context.Background()

	client, err := gcsClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	nameLower := strings.ToLower(*name)
	var files []objectEntry

	it := bucket(client).Objects(ctx, &storage.Query{Prefix: prefix()})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("list objects: %w", err)
		}
		shortName := strings.TrimPrefix(attrs.Name, prefix())
		if strings.Contains(strings.ToLower(shortName), nameLower) {
			files = append(files, objectEntry{
				Key:        attrs.Name,
				Name:       shortName,
				Size:       attrs.Size,
				Updated:    attrs.Updated.UTC().Format(time.RFC3339),
				Generation: attrs.Generation,
			})
		}
	}

	out := struct {
		Files     []objectEntry `json:"files"`
		ElapsedMs int64         `json:"_elapsed_ms"`
	}{Files: files, ElapsedMs: time.Since(start).Milliseconds()}

	if out.Files == nil {
		out.Files = []objectEntry{}
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
