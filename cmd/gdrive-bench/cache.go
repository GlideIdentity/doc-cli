package main

import (
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/api/drive/v3"
)

type cacheEntry struct {
	content      string
	name         string
	etag         string
	modifiedTime string
	fetchedAt    time.Time
}

type contentCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func newContentCache() *contentCache {
	return &contentCache{
		entries: make(map[string]cacheEntry),
	}
}

func (c *contentCache) get(id string) (cacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[id]
	return e, ok
}

func (c *contentCache) set(id, content, name, etag, modifiedTime string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[id] = cacheEntry{
		content:      content,
		name:         name,
		etag:         etag,
		modifiedTime: modifiedTime,
		fetchedAt:    time.Now(),
	}
}

// validateAndGet checks the file's current modifiedTime via a lightweight API call.
// If unchanged, returns cached content. If changed, re-fetches content.
// Returns (content, name, source, error).
func (c *contentCache) validateAndGet(svc *drive.Service, id string) (string, string, string, error) {
	cached, hasCached := c.get(id)

	if hasCached {
		// Lightweight metadata check — only fetches modifiedTime
		meta, err := svc.Files.Get(id).Fields("modifiedTime, name").Do()
		if err != nil {
			return "", "", "", fmt.Errorf("validate: %w", err)
		}

		if meta.ModifiedTime == cached.modifiedTime {
			return cached.content, cached.name, "cache-validated", nil
		}

		// File changed — re-fetch content
		content, err := exportPlainText(svc, id)
		if err != nil {
			return "", "", "", err
		}
		c.set(id, content, meta.Name, "", meta.ModifiedTime)
		return content, meta.Name, "cache-invalidated", nil
	}

	// No cache entry — fetch everything
	meta, err := svc.Files.Get(id).Fields("modifiedTime, name").Do()
	if err != nil {
		return "", "", "", fmt.Errorf("get metadata: %w", err)
	}
	content, err := exportPlainText(svc, id)
	if err != nil {
		return "", "", "", err
	}
	c.set(id, content, meta.Name, "", meta.ModifiedTime)
	return content, meta.Name, "api", nil
}

func exportPlainText(svc *drive.Service, id string) (string, error) {
	resp, err := svc.Files.Export(id, "text/plain").Download()
	if err != nil {
		return "", fmt.Errorf("export: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	return string(b), nil
}

// prefetchContent fetches file content in the background and caches it
func (c *contentCache) prefetchContent(svc *drive.Service, fileID, fileName, modifiedTime string) {
	go func() {
		content, err := exportPlainText(svc, fileID)
		if err != nil {
			return
		}
		c.set(fileID, content, fileName, "", modifiedTime)
	}()
}

// localIndex holds file metadata for instant find/search
type localIndex struct {
	mu    sync.RWMutex
	files []indexedFile
	built bool
}

type indexedFile struct {
	ID           string
	Name         string
	MimeType     string
	ModifiedTime string
	Content      string // only populated after content is fetched
}

func newLocalIndex() *localIndex {
	return &localIndex{}
}

func (idx *localIndex) build(svc *drive.Service, folder string) error {
	q := "trashed = false"
	if folder != "" {
		q += fmt.Sprintf(" and '%s' in parents", folder)
	}

	list, err := svc.Files.List().
		Q(q).
		Fields("files(id, name, mimeType, modifiedTime)").
		PageSize(100).
		Do()
	if err != nil {
		return err
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.files = make([]indexedFile, len(list.Files))
	for i, f := range list.Files {
		idx.files[i] = indexedFile{
			ID:           f.Id,
			Name:         f.Name,
			MimeType:     f.MimeType,
			ModifiedTime: f.ModifiedTime,
		}
	}
	idx.built = true

	// Prefetch content for all files in the background
	for i, f := range idx.files {
		go func(fileID string, fileIdx int) {
			resp, err := svc.Files.Export(fileID, "text/plain").Download()
			if err != nil {
				return
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)

			idx.mu.Lock()
			if fileIdx < len(idx.files) {
				idx.files[fileIdx].Content = string(b)
			}
			idx.mu.Unlock()
		}(f.ID, i)
	}

	return nil
}

func (idx *localIndex) findByName(name string) []indexedFile {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []indexedFile
	nameLower := toLower(name)
	for _, f := range idx.files {
		if containsLower(f.Name, nameLower) {
			results = append(results, f)
		}
	}
	return results
}

func (idx *localIndex) searchContent(query string) []indexedFile {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	queryLower := toLower(query)
	var results []indexedFile
	for _, f := range idx.files {
		if containsLower(f.Content, queryLower) || containsLower(f.Name, queryLower) {
			results = append(results, f)
		}
	}
	return results
}

func (idx *localIndex) isReady() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.built
}

func (idx *localIndex) isContentReady() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if !idx.built {
		return false
	}
	for _, f := range idx.files {
		if f.Content == "" {
			return false
		}
	}
	return true
}

// simple case-insensitive helpers (avoid importing strings in hot path)
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func containsLower(s, sub string) bool {
	s = toLower(s)
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
