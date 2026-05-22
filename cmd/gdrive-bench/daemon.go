package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"google.golang.org/api/drive/v3"
)

func socketPath() string {
	return filepath.Join(configDir(), "daemon.sock")
}

func pidFilePath() string {
	return filepath.Join(configDir(), "daemon.pid")
}

func runDaemon(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: gdrive-bench daemon [start|stop|status]")
	}

	switch args[0] {
	case "start":
		return daemonStart()
	case "stop":
		return daemonStop()
	case "status":
		return daemonStatus()
	default:
		return fmt.Errorf("unknown daemon command: %s", args[0])
	}
}

func daemonStart() error {
	sock := socketPath()
	os.Remove(sock)

	svc, err := driveService()
	if err != nil {
		return fmt.Errorf("pre-warm failed: %w", err)
	}

	// Warm the connection pool
	_, _ = svc.About.Get().Fields("user").Do()

	// Initialize cache and local index
	cache := newContentCache()
	index := newLocalIndex()

	// Build local index in background (fetches file list + prefetches content)
	go func() {
		if err := index.build(svc, folderID()); err != nil {
			fmt.Fprintf(os.Stderr, "index build failed: %v\n", err)
			return
		}
		// Wait for content to be fully fetched, then populate cache
		for i := 0; i < 60; i++ {
			if index.isContentReady() {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		index.mu.RLock()
		for _, f := range index.files {
			if f.Content != "" {
				cache.set(f.ID, f.Content, f.Name, "", f.ModifiedTime)
			}
		}
		index.mu.RUnlock()
	}()

	audit, err := newAuditLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: audit log disabled: %v\n", err)
	}

	listener, err := net.Listen("unix", sock)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	os.WriteFile(pidFilePath(), []byte(fmt.Sprintf("%d", os.Getpid())), 0600)

	mux := http.NewServeMux()
	mux.HandleFunc("/find", daemonFindHandler(svc, cache, index, audit))
	mux.HandleFunc("/read", daemonReadHandler(svc, cache, audit))
	mux.HandleFunc("/search", daemonSearchHandler(svc, index, audit))
	mux.HandleFunc("/create", daemonCreateHandler(svc, audit))
	mux.HandleFunc("/update", daemonUpdateHandler(svc, cache, audit))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "index_ready": index.isReady(), "content_cached": index.isContentReady()})
	})

	server := &http.Server{Handler: mux}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		audit.close()
		server.Shutdown(context.Background())
		os.Remove(sock)
		os.Remove(pidFilePath())
	}()

	fmt.Printf(`{"status": "daemon started", "socket": %q, "pid": %d}`+"\n", sock, os.Getpid())
	return server.Serve(listener)
}

func daemonStop() error {
	data, err := os.ReadFile(pidFilePath())
	if err != nil {
		return fmt.Errorf("daemon not running (no pid file)")
	}
	var pid int
	fmt.Sscanf(string(data), "%d", &pid)

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}
	proc.Signal(syscall.SIGTERM)
	os.Remove(socketPath())
	os.Remove(pidFilePath())
	fmt.Printf(`{"status": "stopped", "pid": %d}`+"\n", pid)
	return nil
}

func daemonStatus() error {
	conn, err := net.Dial("unix", socketPath())
	if err != nil {
		fmt.Println(`{"status": "not running"}`)
		return nil
	}
	conn.Close()
	fmt.Println(`{"status": "running"}`)
	return nil
}

// --- Handler factories (reuse the pre-warmed *drive.Service) ---

func daemonFindHandler(svc *drive.Service, cache *contentCache, index *localIndex, audit *auditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		name := r.URL.Query().Get("name")
		folder := r.URL.Query().Get("folder")
		if folder == "" {
			folder = folderID()
		}

		type entry struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			MimeType     string `json:"mimeType"`
			ModifiedTime string `json:"modifiedTime"`
		}

		if index.isReady() {
			results := index.findByName(name)
			files := make([]entry, len(results))
			for i, f := range results {
				files[i] = entry{f.ID, f.Name, f.MimeType, f.ModifiedTime}
				cache.prefetchContent(svc, f.ID, f.Name, f.ModifiedTime)
			}
			elapsed := time.Since(start).Milliseconds()
			audit.log("find", "", name, "index", "ok", fmt.Sprintf("found %d files", len(files)), elapsed)
			json.NewEncoder(w).Encode(map[string]any{
				"files":      files,
				"_elapsed_ms": elapsed,
				"_source":    "index",
			})
			return
		}

		q := fmt.Sprintf("name contains '%s' and trashed = false", name)
		if folder != "" {
			q += fmt.Sprintf(" and '%s' in parents", folder)
		}

		list, err := svc.Files.List().Q(q).Fields("files(id, name, mimeType, modifiedTime)").PageSize(50).Do()
		if err != nil {
			audit.log("find", "", name, "api", "error", err.Error(), time.Since(start).Milliseconds())
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
			return
		}

		files := make([]entry, len(list.Files))
		for i, f := range list.Files {
			files[i] = entry{f.Id, f.Name, f.MimeType, f.ModifiedTime}
			cache.prefetchContent(svc, f.Id, f.Name, f.ModifiedTime)
		}
		elapsed := time.Since(start).Milliseconds()
		audit.log("find", "", name, "api", "ok", fmt.Sprintf("found %d files", len(files)), elapsed)
		json.NewEncoder(w).Encode(map[string]any{
			"files":      files,
			"_elapsed_ms": elapsed,
			"_source":    "api",
		})
	}
}

func daemonReadHandler(svc *drive.Service, cache *contentCache, audit *auditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := r.URL.Query().Get("id")

		content, name, source, err := cache.validateAndGet(svc, id)
		if err != nil {
			audit.log("read", id, "", "", "error", err.Error(), time.Since(start).Milliseconds())
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
			return
		}

		elapsed := time.Since(start).Milliseconds()
		audit.log("read", id, name, source, "ok", fmt.Sprintf("len=%d", len(content)), elapsed)
		json.NewEncoder(w).Encode(map[string]any{
			"name":        name,
			"content":     content,
			"_elapsed_ms": elapsed,
			"_source":     source,
		})
	}
}

func daemonSearchHandler(svc *drive.Service, index *localIndex, audit *auditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		query := r.URL.Query().Get("query")
		folder := r.URL.Query().Get("folder")
		if folder == "" {
			folder = folderID()
		}

		type entry struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			MimeType     string `json:"mimeType"`
			ModifiedTime string `json:"modifiedTime"`
		}

		if index.isContentReady() {
			results := index.searchContent(query)
			files := make([]entry, len(results))
			for i, f := range results {
				files[i] = entry{f.ID, f.Name, f.MimeType, f.ModifiedTime}
			}
			elapsed := time.Since(start).Milliseconds()
			audit.log("search", "", query, "index", "ok", fmt.Sprintf("found %d files", len(files)), elapsed)
			json.NewEncoder(w).Encode(map[string]any{
				"files":      files,
				"count":      len(files),
				"_elapsed_ms": elapsed,
				"_source":    "index",
			})
			return
		}

		q := fmt.Sprintf("fullText contains '%s' and trashed = false", query)
		if folder != "" {
			q += fmt.Sprintf(" and '%s' in parents", folder)
		}

		list, err := svc.Files.List().Q(q).Fields("files(id, name, mimeType, modifiedTime)").PageSize(50).Do()
		if err != nil {
			audit.log("search", "", query, "api", "error", err.Error(), time.Since(start).Milliseconds())
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
			return
		}

		files := make([]entry, len(list.Files))
		for i, f := range list.Files {
			files[i] = entry{f.Id, f.Name, f.MimeType, f.ModifiedTime}
		}
		elapsed := time.Since(start).Milliseconds()
		audit.log("search", "", query, "api", "ok", fmt.Sprintf("found %d files", len(files)), elapsed)
		json.NewEncoder(w).Encode(map[string]any{
			"files":      files,
			"count":      len(files),
			"_elapsed_ms": elapsed,
			"_source":    "api",
		})
	}
}

func daemonCreateHandler(svc *drive.Service, audit *auditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var req struct {
			Name   string `json:"name"`
			Body   string `json:"body"`
			Folder string `json:"folder"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Folder == "" {
			req.Folder = folderID()
		}

		meta := &drive.File{
			Name:     req.Name,
			MimeType: "application/vnd.google-apps.document",
		}
		if req.Folder != "" {
			meta.Parents = []string{req.Folder}
		}

		created, err := svc.Files.Create(meta).Fields("id, name").Do()
		if err != nil {
			audit.log("create", "", req.Name, "api", "error", err.Error(), time.Since(start).Milliseconds())
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
			return
		}

		if req.Body != "" {
			_, err = svc.Files.Update(created.Id, nil).
				Media(strReader(req.Body)).Do()
			if err != nil {
				audit.log("create", created.Id, req.Name, "api", "error", "content write failed: "+err.Error(), time.Since(start).Milliseconds())
				http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
				return
			}
		}

		elapsed := time.Since(start).Milliseconds()
		audit.log("create", created.Id, created.Name, "api", "ok", "", elapsed)
		json.NewEncoder(w).Encode(map[string]any{
			"id":          created.Id,
			"name":        created.Name,
			"_elapsed_ms": elapsed,
		})
	}
}

func daemonUpdateHandler(svc *drive.Service, cache *contentCache, audit *auditLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var req struct {
			ID             string `json:"id"`
			Body           string `json:"body"`
			Replace        bool   `json:"replace"`
			ExpectModified string `json:"expect_modified"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// Get current file state for concurrency check
		file, err := svc.Files.Get(req.ID).Fields("modifiedTime, name").Do()
		if err != nil {
			audit.log("update", req.ID, "", "api", "error", err.Error(), time.Since(start).Milliseconds())
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
			return
		}

		// Optimistic concurrency: reject if file was modified since agent last read it
		if req.ExpectModified != "" && file.ModifiedTime != req.ExpectModified {
			elapsed := time.Since(start).Milliseconds()
			audit.log("update", req.ID, file.Name, "api", "conflict",
				fmt.Sprintf("expected=%s actual=%s", req.ExpectModified, file.ModifiedTime), elapsed)
			json.NewEncoder(w).Encode(map[string]any{
				"id":                req.ID,
				"name":              file.Name,
				"status":            "conflict",
				"error":             "file was modified by another user since you last read it — re-read before updating",
				"expected_modified": req.ExpectModified,
				"actual_modified":   file.ModifiedTime,
				"_elapsed_ms":       elapsed,
			})
			return
		}

		var content string
		if req.Replace {
			content = req.Body
		} else {
			resp, err := svc.Files.Export(req.ID, "text/plain").Download()
			if err != nil {
				audit.log("update", req.ID, file.Name, "api", "error", "export failed: "+err.Error(), time.Since(start).Milliseconds())
				http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
				return
			}
			defer resp.Body.Close()
			b, _ := readAll(resp.Body)
			content = string(b) + "\n\n" + req.Body
		}

		_, err = svc.Files.Update(req.ID, nil).
			Media(strReader(content)).Do()
		if err != nil {
			audit.log("update", req.ID, file.Name, "api", "error", err.Error(), time.Since(start).Milliseconds())
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), 500)
			return
		}

		// Invalidate cache since we just modified the file
		cache.set(req.ID, content, file.Name, "", time.Now().UTC().Format(time.RFC3339))

		elapsed := time.Since(start).Milliseconds()
		audit.log("update", req.ID, file.Name, "api", "ok", "", elapsed)
		json.NewEncoder(w).Encode(map[string]any{
			"id":          req.ID,
			"name":        file.Name,
			"status":      "updated",
			"_elapsed_ms": elapsed,
		})
	}
}
