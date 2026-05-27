package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type cacheEntry struct {
	content    string
	name       string
	generation int64
	updated    string
	fetchedAt  time.Time
}

type contentCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func newContentCache() *contentCache {
	return &contentCache{entries: make(map[string]cacheEntry)}
}

func (c *contentCache) get(key string) (cacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	return e, ok
}

func (c *contentCache) set(key, content, name string, generation int64, updated string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry{
		content:    content,
		name:       name,
		generation: generation,
		updated:    updated,
		fetchedAt:  time.Now(),
	}
}

// validateAndGet checks the object's generation via a lightweight Attrs call.
func (c *contentCache) validateAndGet(ctx context.Context, bkt *storage.BucketHandle, key string) (string, string, int64, string, error) {
	cached, hasCached := c.get(key)

	attrs, err := bkt.Object(key).Attrs(ctx)
	if err != nil {
		return "", "", 0, "", fmt.Errorf("attrs: %w", err)
	}

	if hasCached && attrs.Generation == cached.generation {
		return cached.content, cached.name, cached.generation, "cache-validated", nil
	}

	// Generation changed or cache miss — re-fetch
	reader, err := bkt.Object(key).NewReader(ctx)
	if err != nil {
		return "", "", 0, "", fmt.Errorf("read: %w", err)
	}
	defer reader.Close()
	b, _ := io.ReadAll(reader)

	name := strings.TrimPrefix(key, prefix())
	updated := attrs.Updated.UTC().Format(time.RFC3339)
	c.set(key, string(b), name, attrs.Generation, updated)

	source := "api"
	if hasCached {
		source = "cache-invalidated"
	}
	return string(b), name, attrs.Generation, source, nil
}

func (c *contentCache) prefetch(ctx context.Context, bkt *storage.BucketHandle, key, name string, generation int64, updated string) {
	go func() {
		reader, err := bkt.Object(key).NewReader(ctx)
		if err != nil {
			return
		}
		defer reader.Close()
		b, _ := io.ReadAll(reader)
		c.set(key, string(b), name, generation, updated)
	}()
}

// localIndex for instant find/search
type localIndex struct {
	mu    sync.RWMutex
	files []indexedObject
	built bool
}

type indexedObject struct {
	Key        string
	Name       string
	Size       int64
	Updated    string
	Generation int64
	Content    string
}

func newLocalIndex() *localIndex {
	return &localIndex{}
}

func (idx *localIndex) build(ctx context.Context, bkt *storage.BucketHandle) error {
	it := bkt.Objects(ctx, &storage.Query{Prefix: prefix()})

	idx.mu.Lock()
	idx.files = nil

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			idx.mu.Unlock()
			return err
		}
		idx.files = append(idx.files, indexedObject{
			Key:        attrs.Name,
			Name:       strings.TrimPrefix(attrs.Name, prefix()),
			Size:       attrs.Size,
			Updated:    attrs.Updated.UTC().Format(time.RFC3339),
			Generation: attrs.Generation,
		})
	}
	idx.built = true
	idx.mu.Unlock()

	// Prefetch content for all objects
	for i, f := range idx.files {
		go func(key string, fileIdx int) {
			reader, err := bkt.Object(key).NewReader(ctx)
			if err != nil {
				return
			}
			defer reader.Close()
			b, _ := io.ReadAll(reader)

			idx.mu.Lock()
			if fileIdx < len(idx.files) {
				idx.files[fileIdx].Content = string(b)
			}
			idx.mu.Unlock()
		}(f.Key, i)
	}

	return nil
}

func (idx *localIndex) findByName(name string) []indexedObject {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	nameLower := strings.ToLower(name)
	var results []indexedObject
	for _, f := range idx.files {
		if strings.Contains(strings.ToLower(f.Name), nameLower) {
			results = append(results, f)
		}
	}
	return results
}

func (idx *localIndex) searchContent(query string) []indexedObject {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	queryLower := strings.ToLower(query)
	var results []indexedObject
	for _, f := range idx.files {
		if strings.Contains(strings.ToLower(f.Content), queryLower) ||
			strings.Contains(strings.ToLower(f.Name), queryLower) {
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
	if !idx.built || len(idx.files) == 0 {
		return false
	}
	for _, f := range idx.files {
		if f.Content == "" {
			return false
		}
	}
	return true
}
