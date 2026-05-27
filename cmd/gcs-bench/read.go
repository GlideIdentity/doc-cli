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
)

func runRead(args []string) error {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	key := fs.String("key", "", "object key (or short name)")
	fs.Parse(args)

	if *key == "" {
		return fmt.Errorf("--key is required")
	}

	start := time.Now()
	ctx := context.Background()

	client, err := gcsClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	fullKey := *key
	if !strings.HasPrefix(fullKey, prefix()) && prefix() != "" {
		fullKey = objectKey(fullKey)
	}

	obj := bucket(client).Object(fullKey)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("get attrs: %w", err)
	}

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	defer reader.Close()

	b, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	out := struct {
		Key        string `json:"key"`
		Name       string `json:"name"`
		Content    string `json:"content"`
		Generation int64  `json:"generation"`
		ElapsedMs  int64  `json:"_elapsed_ms"`
	}{
		Key:        fullKey,
		Name:       strings.TrimPrefix(fullKey, prefix()),
		Content:    string(b),
		Generation: attrs.Generation,
		ElapsedMs:  time.Since(start).Milliseconds(),
	}

	return json.NewEncoder(os.Stdout).Encode(out)
}
