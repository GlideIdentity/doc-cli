package main

import (
	"testing"
)

// --- contentCache ---

func TestContentCache_SetAndGet(t *testing.T) {
	c := newContentCache()

	c.set("key1", "hello world", "file.md", 100, "2025-01-01T00:00:00Z")
	entry, ok := c.get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if entry.content != "hello world" {
		t.Errorf("content: got %q, want %q", entry.content, "hello world")
	}
	if entry.name != "file.md" {
		t.Errorf("name: got %q, want %q", entry.name, "file.md")
	}
	if entry.generation != 100 {
		t.Errorf("generation: got %d, want %d", entry.generation, 100)
	}
	if entry.updated != "2025-01-01T00:00:00Z" {
		t.Errorf("updated: got %q, want %q", entry.updated, "2025-01-01T00:00:00Z")
	}
	if entry.fetchedAt.IsZero() {
		t.Error("fetchedAt should be set")
	}
}

func TestContentCache_MissReturnsNotOk(t *testing.T) {
	c := newContentCache()

	_, ok := c.get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestContentCache_OverwriteEntry(t *testing.T) {
	c := newContentCache()

	c.set("key1", "version1", "file.md", 1, "2025-01-01T00:00:00Z")
	c.set("key1", "version2", "file.md", 2, "2025-01-02T00:00:00Z")

	entry, ok := c.get("key1")
	if !ok {
		t.Fatal("expected cache hit after overwrite")
	}
	if entry.content != "version2" {
		t.Errorf("content: got %q, want %q", entry.content, "version2")
	}
	if entry.generation != 2 {
		t.Errorf("generation: got %d, want %d", entry.generation, 2)
	}
}

func TestContentCache_MultipleKeys(t *testing.T) {
	c := newContentCache()

	c.set("a", "aaa", "a.md", 1, "2025-01-01T00:00:00Z")
	c.set("b", "bbb", "b.md", 2, "2025-01-02T00:00:00Z")
	c.set("c", "ccc", "c.md", 3, "2025-01-03T00:00:00Z")

	for _, tc := range []struct {
		key, content string
	}{
		{"a", "aaa"},
		{"b", "bbb"},
		{"c", "ccc"},
	} {
		entry, ok := c.get(tc.key)
		if !ok {
			t.Errorf("key=%s: expected cache hit", tc.key)
			continue
		}
		if entry.content != tc.content {
			t.Errorf("key=%s: got %q, want %q", tc.key, entry.content, tc.content)
		}
	}
}

func TestContentCache_EmptyContent(t *testing.T) {
	c := newContentCache()

	c.set("empty", "", "empty.md", 1, "2025-01-01T00:00:00Z")
	entry, ok := c.get("empty")
	if !ok {
		t.Fatal("expected cache hit for empty content")
	}
	if entry.content != "" {
		t.Errorf("content: got %q, want empty string", entry.content)
	}
}

// --- localIndex findByName ---

func makeTestIndex(files []indexedObject) *localIndex {
	idx := newLocalIndex()
	idx.files = files
	idx.built = true
	return idx
}

func TestLocalIndex_FindByName_ExactMatch(t *testing.T) {
	idx := makeTestIndex([]indexedObject{
		{Key: "prefix/notes.md", Name: "notes.md", Size: 100, Generation: 1},
		{Key: "prefix/readme.md", Name: "readme.md", Size: 200, Generation: 2},
		{Key: "prefix/todo.md", Name: "todo.md", Size: 50, Generation: 3},
	})

	results := idx.findByName("notes.md")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "notes.md" {
		t.Errorf("name: got %q, want %q", results[0].Name, "notes.md")
	}
}

func TestLocalIndex_FindByName_PartialMatch(t *testing.T) {
	idx := makeTestIndex([]indexedObject{
		{Key: "prefix/project-notes.md", Name: "project-notes.md"},
		{Key: "prefix/meeting-notes.md", Name: "meeting-notes.md"},
		{Key: "prefix/readme.md", Name: "readme.md"},
	})

	results := idx.findByName("notes")
	if len(results) != 2 {
		t.Fatalf("expected 2 results for partial match, got %d", len(results))
	}
}

func TestLocalIndex_FindByName_CaseInsensitive(t *testing.T) {
	idx := makeTestIndex([]indexedObject{
		{Key: "prefix/README.md", Name: "README.md"},
		{Key: "prefix/readme.txt", Name: "readme.txt"},
	})

	results := idx.findByName("readme")
	if len(results) != 2 {
		t.Fatalf("expected 2 case-insensitive results, got %d", len(results))
	}

	results = idx.findByName("README")
	if len(results) != 2 {
		t.Fatalf("expected 2 case-insensitive results for uppercase, got %d", len(results))
	}
}

func TestLocalIndex_FindByName_NoMatch(t *testing.T) {
	idx := makeTestIndex([]indexedObject{
		{Key: "prefix/notes.md", Name: "notes.md"},
	})

	results := idx.findByName("nonexistent")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestLocalIndex_FindByName_EmptyIndex(t *testing.T) {
	idx := makeTestIndex(nil)

	results := idx.findByName("anything")
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty index, got %d", len(results))
	}
}

// --- localIndex searchContent ---

func TestLocalIndex_SearchContent_MatchesContent(t *testing.T) {
	idx := makeTestIndex([]indexedObject{
		{Key: "a", Name: "a.md", Content: "This document covers Go testing best practices."},
		{Key: "b", Name: "b.md", Content: "Python tutorial for beginners."},
		{Key: "c", Name: "c.md", Content: "Advanced Go concurrency patterns."},
	})

	results := idx.searchContent("Go")
	if len(results) != 2 {
		t.Fatalf("expected 2 results matching 'Go', got %d", len(results))
	}
}

func TestLocalIndex_SearchContent_MatchesName(t *testing.T) {
	idx := makeTestIndex([]indexedObject{
		{Key: "a", Name: "golang-guide.md", Content: "Some content here."},
		{Key: "b", Name: "python-guide.md", Content: "Other content."},
	})

	results := idx.searchContent("golang")
	if len(results) != 1 {
		t.Fatalf("expected 1 result matching name 'golang', got %d", len(results))
	}
	if results[0].Name != "golang-guide.md" {
		t.Errorf("expected golang-guide.md, got %q", results[0].Name)
	}
}

func TestLocalIndex_SearchContent_CaseInsensitive(t *testing.T) {
	idx := makeTestIndex([]indexedObject{
		{Key: "a", Name: "a.md", Content: "IMPORTANT: Read this document carefully."},
		{Key: "b", Name: "b.md", Content: "This is important too."},
		{Key: "c", Name: "c.md", Content: "Nothing special here."},
	})

	results := idx.searchContent("important")
	if len(results) != 2 {
		t.Fatalf("expected 2 case-insensitive results, got %d", len(results))
	}

	results = idx.searchContent("IMPORTANT")
	if len(results) != 2 {
		t.Fatalf("expected 2 results for uppercase query, got %d", len(results))
	}
}

func TestLocalIndex_SearchContent_NoMatch(t *testing.T) {
	idx := makeTestIndex([]indexedObject{
		{Key: "a", Name: "a.md", Content: "Hello world"},
	})

	results := idx.searchContent("xyz123nonexistent")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestLocalIndex_SearchContent_EmptyQuery(t *testing.T) {
	idx := makeTestIndex([]indexedObject{
		{Key: "a", Name: "a.md", Content: "Hello"},
		{Key: "b", Name: "b.md", Content: "World"},
	})

	results := idx.searchContent("")
	if len(results) != 2 {
		t.Errorf("empty query should match all, got %d results", len(results))
	}
}

func TestLocalIndex_SearchContent_MatchesBothNameAndContent(t *testing.T) {
	idx := makeTestIndex([]indexedObject{
		{Key: "a", Name: "testing-guide.md", Content: "This guide covers testing in Go."},
	})

	results := idx.searchContent("testing")
	if len(results) != 1 {
		t.Fatalf("expected 1 result (no duplicates), got %d", len(results))
	}
}

// --- localIndex status helpers ---

func TestLocalIndex_IsReady(t *testing.T) {
	idx := newLocalIndex()
	if idx.isReady() {
		t.Error("new index should not be ready")
	}

	idx.mu.Lock()
	idx.built = true
	idx.mu.Unlock()

	if !idx.isReady() {
		t.Error("index should be ready after build")
	}
}

func TestLocalIndex_IsContentReady(t *testing.T) {
	idx := newLocalIndex()
	if idx.isContentReady() {
		t.Error("new index should not have content ready")
	}

	idx.mu.Lock()
	idx.built = true
	idx.files = []indexedObject{
		{Key: "a", Name: "a.md", Content: ""},
	}
	idx.mu.Unlock()

	if idx.isContentReady() {
		t.Error("index with empty content should not be content-ready")
	}

	idx.mu.Lock()
	idx.files[0].Content = "populated"
	idx.mu.Unlock()

	if !idx.isContentReady() {
		t.Error("index with all content populated should be content-ready")
	}
}

func TestLocalIndex_IsContentReady_EmptyFiles(t *testing.T) {
	idx := newLocalIndex()
	idx.mu.Lock()
	idx.built = true
	idx.files = []indexedObject{}
	idx.mu.Unlock()

	if idx.isContentReady() {
		t.Error("built index with zero files should report not content-ready")
	}
}
