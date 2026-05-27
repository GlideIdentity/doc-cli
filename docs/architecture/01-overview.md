# Architecture Overview

## What This Is

A benchmarking toolkit comparing three approaches for AI agents to access documents:

1. **Google Drive CLI** (`gdrive-bench`) — native Go binary calling Drive API
2. **GCS CLI** (`gcs-bench`) — native Go binary calling Cloud Storage API
3. **Local Markdown Files** — agent reads files directly from disk

Each CLI includes a persistent daemon with connection pooling, local indexing, content caching, ETag-based freshness validation, optimistic concurrency control, and audit logging.

## System Diagram

```
                        ┌─────────────────────────────────────┐
                        │          Cursor Agent               │
                        │                                     │
                        │  ┌───────────┐  ┌────────────────┐  │
                        │  │ CLI Skill │  │ local-docs     │  │
                        │  │ (gdrive/  │  │ Skill          │  │
                        │  │  gcs)     │  │                │  │
                        │  └─────┬─────┘  └───────┬────────┘  │
                        └────────┼────────────────┼───────────┘
                                 │                │
                    ┌────────────▼──────┐   ┌─────▼──────┐
                    │   CLI Binary      │   │ Local FS   │
                    │                   │   │ (Read/     │
                    │ ┌───────────────┐ │   │  Write/    │
                    │ │ Daemon        │ │   │  Grep)     │
                    │ │ ┌───────────┐ │ │   └────────────┘
                    │ │ │ Cache     │ │ │        <1ms
                    │ │ │ Index     │ │ │
                    │ │ │ Audit Log │ │ │
                    │ │ │ CAS       │ │ │
                    │ │ └───────────┘ │ │
                    │ └───────┬───────┘ │
                    └─────────┼─────────┘
                              │ Unix Socket
                              │
               ┌──────────────┼──────────────┐
               │              │              │
        ┌──────▼──────┐ ┌────▼─────┐  ┌─────▼─────┐
        │ Google Drive│ │ GCS      │  │ (cached   │
        │ API (REST)  │ │ API      │  │  in-memory│
        │             │ │ (gRPC)   │  │  response)│
        │ ~500ms/call │ │ ~50ms    │  │  0ms      │
        └─────────────┘ └──────────┘  └───────────┘
```

## Performance Summary (Measured)

| Operation | Drive CLI (original) | Drive CLI (optimized) | GCS CLI (target) | Local Files |
|-----------|---------------------|----------------------|------------------|-------------|
| Find      | 1,933ms             | 0-12ms (index)       | 0ms (index)      | <1ms        |
| Read      | 6,604ms             | 500ms (validated)    | ~50ms (validated) | <1ms        |
| Search    | 1,199ms             | 0-13ms (index)       | 0ms (index)      | 3ms         |
| Create    | ~3,000ms            | ~3,000ms (API)       | ~100ms (API)     | ~1ms        |
| Update    | ~3,000ms            | ~3,000ms (API)       | ~100ms (API)     | ~1ms        |

## Project Structure

```
clitest/
├── cmd/
│   ├── gdrive-bench/          # Google Drive CLI (16 Go files)
│   │   ├── main.go            # Entrypoint, subcommand routing, daemon dispatch
│   │   ├── auth.go            # OAuth2 flow, token management
│   │   ├── daemon.go          # HTTP server on Unix socket
│   │   ├── cache.go           # Content cache + local index + ETag validation
│   │   ├── audit.go           # JSONL audit logger
│   │   ├── client.go          # Unix socket client for daemon communication
│   │   ├── find.go            # Find files by name
│   │   ├── read.go            # Read/export file content
│   │   ├── search.go          # Full-text search
│   │   ├── create.go          # Create new document
│   │   └── update.go          # Update with optimistic concurrency
│   └── gcs-bench/             # GCS CLI (12 Go files)
│       ├── main.go            # Same structure as gdrive-bench
│       ├── config.go          # Bucket/prefix config, SA key auto-detection
│       ├── daemon.go          # Same daemon architecture
│       ├── cache.go           # Generation-based cache validation
│       ├── audit.go           # JSONL audit logger
│       └── ...                # find, read, search, create, update
├── docs/                      # Test documents (12 markdown files)
├── skills/                    # Cursor agent skills
│   ├── gdrive-cli/SKILL.md
│   ├── gcs-cli/SKILL.md
│   └── local-docs/SKILL.md
├── benchmark/
│   ├── protocol.md            # 5-task benchmark protocol
│   ├── analyze.ts             # Results analysis script
│   └── results/               # Raw benchmark JSON
└── scripts/
    ├── raw-benchmark.ts       # 3-way automated benchmark
    └── upload-docs.sh         # Upload docs to Drive
```
