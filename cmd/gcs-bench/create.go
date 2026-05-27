package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func runCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	name := fs.String("name", "", "object name")
	body := fs.String("body", "", "content")
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

	key := objectKey(*name)
	writer := bucket(client).Object(key).NewWriter(ctx)
	writer.ContentType = "text/markdown"

	if _, err := writer.Write([]byte(*body)); err != nil {
		writer.Close()
		return fmt.Errorf("write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close writer: %w", err)
	}

	out := struct {
		Key       string `json:"key"`
		Name      string `json:"name"`
		ElapsedMs int64  `json:"_elapsed_ms"`
	}{
		Key:       key,
		Name:      strings.TrimPrefix(key, prefix()),
		ElapsedMs: time.Since(start).Milliseconds(),
	}

	return json.NewEncoder(os.Stdout).Encode(out)
}
