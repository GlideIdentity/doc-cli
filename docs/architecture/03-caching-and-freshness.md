# Caching and Freshness

## The Problem

There are three conflicting goals:

1. **Speed** — serve responses in <1ms (like local files)
2. **Freshness** — never show stale data when another user edited the file
3. **Cost** — minimize API calls (each costs latency and quota)

No single strategy satisfies all three. This system uses a layered approach.

## Cache Layers

### Layer 1: Local Index (metadata)

On daemon startup, the index fetches the full file list from the API once. Find and Search operations query this in-memory index.

- **Speed**: 0ms (in-memory string matching)
- **Freshness**: Stale after startup (metadata only refreshed on daemon restart)
- **Acceptable for**: file discovery — names/IDs rarely change mid-session

### Layer 2: Content Cache (with ETag/generation validation)

File content is prefetched into an in-memory cache on startup. On every Read, the daemon validates the cache before serving:

```
Read request arrives
    │
    ▼
Cache has entry? ──No──→ Fetch from API, cache, return
    │
   Yes
    │
    ▼
Lightweight metadata check (modifiedTime or generation)
    │
    ├── Unchanged → Return cached content (saved one full download)
    │
    └── Changed → Re-fetch content, update cache, return
```

#### Google Drive: modifiedTime validation

```go
meta, _ := svc.Files.Get(id).Fields("modifiedTime, name").Do()
if meta.ModifiedTime == cached.modifiedTime {
    return cached.content  // cache hit, ~500ms (metadata call only)
}
// cache miss — re-download content, ~900ms
```

#### GCS: generation validation

```go
attrs, _ := bkt.Object(key).Attrs(ctx)
if attrs.Generation == cached.generation {
    return cached.content  // cache hit, ~30-50ms (attrs call only)
}
// cache miss — re-download, ~80ms
```

GCS validation is ~10x faster than Drive because GCS Attrs is a lightweight metadata-only call with no collaboration overhead.

### Layer 3: Prefetch on Find

When the agent runs `find`, it almost always runs `read` next. The daemon speculatively prefetches content for all results:

```go
for _, f := range results {
    cache.prefetch(svc, f.ID, f.Name, f.ModifiedTime)
}
```

This runs in background goroutines. If the agent reads a found file within 1-2 seconds, the content is already cached.

## Freshness Guarantees

| Operation | Source      | Freshness                                    |
|-----------|-------------|----------------------------------------------|
| Find      | Local index | Metadata from daemon startup                 |
| Read      | Cache + API | Always validated — stale data never returned  |
| Search    | Local index | Content from daemon startup (stale risk)      |
| Create    | API         | Always live                                  |
| Update    | API + CAS   | Always live, with conflict detection          |

Read is the only operation with a hard freshness guarantee (ETag/generation check on every call). Find and Search use the startup-time index, which is a deliberate trade-off for speed.

## Cache Entry Structure

```go
type cacheEntry struct {
    content      string    // file content
    name         string    // display name
    generation   int64     // GCS generation number (or 0 for Drive)
    modifiedTime string    // Drive modifiedTime (or "" for GCS)
    fetchedAt    time.Time // when this was cached
}
```

## When to Restart the Daemon

- After files are added or deleted (index doesn't pick up new files)
- After bulk edits by other users (forces full re-index)
- When the daemon has been running for hours and index staleness matters

A future improvement could add `changes.watch` (Drive) or GCS Pub/Sub notifications to refresh the index incrementally without restart.
