package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/storage"
)

func runDaemon(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gcs-bench daemon [start|stop|status]")
	}
	switch args[0] {
	case "start":
		return daemonStart()
	case "stop":
		return daemonStop()
	case "status":
		return daemonStatus()
	default:
		return fmt.Errorf("unknown: %s", args[0])
	}
}

func daemonStart() error {
	sock := socketPath()
	os.Remove(sock)

	ctx := context.Background()
	client, err := gcsClient(ctx)
	if err != nil {
		return fmt.Errorf("gcs client: %w", err)
	}

	bkt := bucket(client)

	// Warm the connection with a quick attrs call
	_, _ = bkt.Attrs(ctx)

	cache := newContentCache()
	index := newLocalIndex()

	audit, err := newAuditLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: audit log disabled: %v\n", err)
	}

	// Build index in background
	go func() {
		if err := index.build(ctx, bkt); err != nil {
			fmt.Fprintf(os.Stderr, "index build failed: %v\n", err)
			return
		}
		for i := 0; i < 60; i++ {
			if index.isContentReady() {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		index.mu.RLock()
		for _, f := range index.files {
			if f.Content != "" {
				cache.set(f.Key, f.Content, f.Name, f.Generation, f.Updated)
			}
		}
		index.mu.RUnlock()
	}()

	listener, err := net.Listen("unix", sock)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	os.WriteFile(pidFilePath(), []byte(fmt.Sprintf("%d", os.Getpid())), 0600)

	mux := http.NewServeMux()
	mux.HandleFunc("/find", handleFind(ctx, bkt, cache, index, audit))
	mux.HandleFunc("/read", handleRead(ctx, bkt, cache, audit))
	mux.HandleFunc("/search", handleSearch(ctx, bkt, index, audit))
	mux.HandleFunc("/create", handleCreate(ctx, bkt, audit))
	mux.HandleFunc("/update", handleUpdate(ctx, bkt, cache, audit))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok", "index_ready": index.isReady(), "content_cached": index.isContentReady(),
		})
	})

	server := &http.Server{Handler: mux}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		audit.close()
		client.Close()
		server.Shutdown(context.Background())
		os.Remove(sock)
		os.Remove(pidFilePath())
	}()

	fmt.Printf(`{"status": "daemon started", "socket": %q, "pid": %d, "bucket": %q}`+"\n", sock, os.Getpid(), bucketName())
	return server.Serve(listener)
}

func daemonStop() error {
	data, err := os.ReadFile(pidFilePath())
	if err != nil {
		return fmt.Errorf("daemon not running")
	}
	var pid int
	fmt.Sscanf(string(data), "%d", &pid)
	proc, _ := os.FindProcess(pid)
	proc.Signal(syscall.SIGTERM)
	os.Remove(socketPath())
	os.Remove(pidFilePath())
	fmt.Printf(`{"status": "stopped", "pid": %d}`+"\n", pid)
	return nil
}

func daemonStatus() error {
	if isDaemonRunning() {
		fmt.Println(`{"status": "running"}`)
	} else {
		fmt.Println(`{"status": "not running"}`)
	}
	return nil
}

// --- Handlers ---

type objEntry struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Updated    string `json:"updated"`
	Generation int64  `json:"generation"`
}

func handleFind(ctx context.Context, bkt *storage.BucketHandle, cache *contentCache, index *localIndex, audit *auditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		name := r.URL.Query().Get("name")

		if index.isReady() {
			results := index.findByName(name)
			files := make([]objEntry, len(results))
			for i, f := range results {
				files[i] = objEntry{f.Key, f.Name, f.Size, f.Updated, f.Generation}
				cache.prefetch(ctx, bkt, f.Key, f.Name, f.Generation, f.Updated)
			}
			elapsed := time.Since(start).Milliseconds()
			audit.log("find", "", name, "index", "ok", fmt.Sprintf("found %d", len(files)), elapsed)
			json.NewEncoder(w).Encode(map[string]any{"files": files, "_elapsed_ms": elapsed, "_source": "index"})
			return
		}

		// Fallback: list from GCS
		elapsed := time.Since(start).Milliseconds()
		audit.log("find", "", name, "api", "ok", "index not ready, fallback", elapsed)
		json.NewEncoder(w).Encode(map[string]any{"files": []objEntry{}, "_elapsed_ms": elapsed, "_source": "api"})
	}
}

func handleRead(ctx context.Context, bkt *storage.BucketHandle, cache *contentCache, audit *auditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		key := r.URL.Query().Get("key")
		if !strings.HasPrefix(key, prefix()) && prefix() != "" {
			key = objectKey(key)
		}

		content, name, gen, source, err := cache.validateAndGet(ctx, bkt, key)
		if err != nil {
			audit.log("read", key, "", "", "error", err.Error(), time.Since(start).Milliseconds())
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
			return
		}

		elapsed := time.Since(start).Milliseconds()
		audit.log("read", key, name, source, "ok", fmt.Sprintf("len=%d gen=%d", len(content), gen), elapsed)
		json.NewEncoder(w).Encode(map[string]any{
			"name": name, "content": content, "generation": gen,
			"_elapsed_ms": elapsed, "_source": source,
		})
	}
}

func handleSearch(ctx context.Context, bkt *storage.BucketHandle, index *localIndex, audit *auditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		query := r.URL.Query().Get("query")

		if index.isContentReady() {
			results := index.searchContent(query)
			files := make([]objEntry, len(results))
			for i, f := range results {
				files[i] = objEntry{f.Key, f.Name, f.Size, f.Updated, f.Generation}
			}
			elapsed := time.Since(start).Milliseconds()
			audit.log("search", "", query, "index", "ok", fmt.Sprintf("found %d", len(files)), elapsed)
			json.NewEncoder(w).Encode(map[string]any{
				"files": files, "count": len(files), "_elapsed_ms": elapsed, "_source": "index",
			})
			return
		}

		elapsed := time.Since(start).Milliseconds()
		audit.log("search", "", query, "api", "ok", "index not ready", elapsed)
		json.NewEncoder(w).Encode(map[string]any{"files": []objEntry{}, "count": 0, "_elapsed_ms": elapsed, "_source": "api"})
	}
}

func handleCreate(ctx context.Context, bkt *storage.BucketHandle, audit *auditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var req struct {
			Name string `json:"name"`
			Body string `json:"body"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		key := objectKey(req.Name)
		writer := bkt.Object(key).NewWriter(ctx)
		writer.ContentType = "text/markdown"
		writer.Write([]byte(req.Body))
		if err := writer.Close(); err != nil {
			audit.log("create", key, req.Name, "api", "error", err.Error(), time.Since(start).Milliseconds())
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
			return
		}

		elapsed := time.Since(start).Milliseconds()
		audit.log("create", key, req.Name, "api", "ok", "", elapsed)
		json.NewEncoder(w).Encode(map[string]any{"key": key, "name": req.Name, "_elapsed_ms": elapsed})
	}
}

func handleUpdate(ctx context.Context, bkt *storage.BucketHandle, cache *contentCache, audit *auditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var req struct {
			Key       string `json:"key"`
			Body      string `json:"body"`
			ExpectGen int64  `json:"expect_gen"`
			Replace   bool   `json:"replace"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		fullKey := req.Key
		if !strings.HasPrefix(fullKey, prefix()) && prefix() != "" {
			fullKey = objectKey(fullKey)
		}

		obj := bkt.Object(fullKey)
		attrs, err := obj.Attrs(ctx)
		if err != nil {
			audit.log("update", fullKey, "", "api", "error", err.Error(), time.Since(start).Milliseconds())
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
			return
		}

		if req.ExpectGen != 0 && attrs.Generation != req.ExpectGen {
			elapsed := time.Since(start).Milliseconds()
			audit.log("update", fullKey, "", "api", "conflict",
				fmt.Sprintf("expected=%d actual=%d", req.ExpectGen, attrs.Generation), elapsed)
			json.NewEncoder(w).Encode(map[string]any{
				"key": fullKey, "status": "conflict",
				"error": "object modified since last read", "expected_gen": req.ExpectGen,
				"actual_gen": attrs.Generation, "_elapsed_ms": elapsed,
			})
			return
		}

		var content string
		if req.Replace {
			content = req.Body
		} else {
			reader, err := obj.NewReader(ctx)
			if err != nil {
				audit.log("update", fullKey, "", "api", "error", err.Error(), time.Since(start).Milliseconds())
				http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
				return
			}
			b, _ := io.ReadAll(reader)
			reader.Close()
			content = string(b) + "\n\n" + req.Body
		}

		condObj := obj.If(storage.Conditions{GenerationMatch: attrs.Generation})
		writer := condObj.NewWriter(ctx)
		writer.ContentType = "text/markdown"
		writer.Write([]byte(content))
		if err := writer.Close(); err != nil {
			if strings.Contains(err.Error(), "conditionNotMet") {
				elapsed := time.Since(start).Milliseconds()
				audit.log("update", fullKey, "", "api", "conflict", "concurrent write", elapsed)
				json.NewEncoder(w).Encode(map[string]any{
					"key": fullKey, "status": "conflict",
					"error": "concurrent modification during write", "_elapsed_ms": elapsed,
				})
				return
			}
			audit.log("update", fullKey, "", "api", "error", err.Error(), time.Since(start).Milliseconds())
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
			return
		}

		name := strings.TrimPrefix(fullKey, prefix())
		cache.set(fullKey, content, name, attrs.Generation+1, time.Now().UTC().Format(time.RFC3339))

		elapsed := time.Since(start).Milliseconds()
		audit.log("update", fullKey, name, "api", "ok", "", elapsed)
		json.NewEncoder(w).Encode(map[string]any{
			"key": fullKey, "name": name, "status": "updated", "_elapsed_ms": elapsed,
		})
	}
}
