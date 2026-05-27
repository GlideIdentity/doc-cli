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
)

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	key := fs.String("key", "", "object key")
	body := fs.String("body", "", "content to append")
	expectGen := fs.Int64("expect-gen", 0, "expected generation for CAS (0 = skip check)")
	replace := fs.Bool("replace", false, "replace instead of append")
	fs.Parse(args)

	if *key == "" {
		return fmt.Errorf("--key is required")
	}
	if *body == "" {
		return fmt.Errorf("--body is required")
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

	// Check current generation for CAS
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("get attrs: %w", err)
	}

	if *expectGen != 0 && attrs.Generation != *expectGen {
		out := struct {
			Key         string `json:"key"`
			Status      string `json:"status"`
			Error       string `json:"error"`
			ExpectedGen int64  `json:"expected_gen"`
			ActualGen   int64  `json:"actual_gen"`
			ElapsedMs   int64  `json:"_elapsed_ms"`
		}{
			Key:         fullKey,
			Status:      "conflict",
			Error:       "object was modified since you last read it",
			ExpectedGen: *expectGen,
			ActualGen:   attrs.Generation,
			ElapsedMs:   time.Since(start).Milliseconds(),
		}
		return json.NewEncoder(os.Stdout).Encode(out)
	}

	var newContent string
	if *replace {
		newContent = *body
	} else {
		reader, err := obj.NewReader(ctx)
		if err != nil {
			return fmt.Errorf("read existing: %w", err)
		}
		b, _ := io.ReadAll(reader)
		reader.Close()
		newContent = string(b) + "\n\n" + *body
	}

	// Write with generation precondition for true CAS
	condObj := obj.If(storage.Conditions{GenerationMatch: attrs.Generation})
	writer := condObj.NewWriter(ctx)
	writer.ContentType = "text/markdown"
	if _, err := writer.Write([]byte(newContent)); err != nil {
		writer.Close()
		return fmt.Errorf("write: %w", err)
	}
	if err := writer.Close(); err != nil {
		if strings.Contains(err.Error(), "conditionNotMet") {
			out := struct {
				Key       string `json:"key"`
				Status    string `json:"status"`
				Error     string `json:"error"`
				ElapsedMs int64  `json:"_elapsed_ms"`
			}{
				Key:       fullKey,
				Status:    "conflict",
				Error:     "concurrent modification detected during write",
				ElapsedMs: time.Since(start).Milliseconds(),
			}
			return json.NewEncoder(os.Stdout).Encode(out)
		}
		return fmt.Errorf("close writer: %w", err)
	}

	out := struct {
		Key       string `json:"key"`
		Name      string `json:"name"`
		Status    string `json:"status"`
		ElapsedMs int64  `json:"_elapsed_ms"`
	}{
		Key:       fullKey,
		Name:      strings.TrimPrefix(fullKey, prefix()),
		Status:    "updated",
		ElapsedMs: time.Since(start).Milliseconds(),
	}

	return json.NewEncoder(os.Stdout).Encode(out)
}
