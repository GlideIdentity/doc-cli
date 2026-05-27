# Deep Dive: Mirage and gcsfuse

---

## 1. Mirage (strukto-ai/mirage)

### What It Is

A unified virtual filesystem library (not a service) that mounts cloud backends — S3, GCS, Google Drive, Slack, GitHub, Linear, Notion, MongoDB, Redis, SSH, and more — behind a single tree. Agents use standard bash commands (`ls`, `cat`, `grep`, `find`, `echo`) across all mounted backends. The shell is a tree-sitter bash parser with a custom executor — it doesn't invoke `/bin/bash`, it routes each command to the appropriate mount handler in-process.

- **First release**: v0.0.1, May 6, 2026
- **Language**: Python (reference) + TypeScript SDK
- **License**: open source
- **Link**: https://github.com/strukto-ai/mirage
- **Docs**: https://docs.mirage.strukto.ai

### How It Works

```
┌─────────────────────────────────────────────┐
│              Workspace                       │
│                                              │
│  /data    → RAMResource (in-memory)         │
│  /s3      → S3Resource (bucket: my-bucket)  │
│  /gcs     → GCSResource (bucket: my-gcs)   │
│  /gdrive  → GDriveResource (folder: ...)   │
│  /slack   → SlackResource (channel: eng)    │
│  /github  → GitHubResource (repo: org/r)   │
│  /linear  → LinearResource (team: ...)      │
│                                              │
│  ┌───────────────────────────────────┐       │
│  │ Tree-sitter Bash Parser          │       │
│  │ + Custom Executor                │       │
│  │                                   │       │
│  │ ws.execute('grep -r "bug" /slack')│       │
│  │ → routes to SlackResource.grep()  │       │
│  └───────────────────────────────────┘       │
│                                              │
│  ┌──────────────┐  ┌──────────────────┐      │
│  │ Index Cache  │  │ File Cache       │      │
│  │ (metadata)   │  │ (object bytes)   │      │
│  │ TTL: 10min   │  │ RAM: 512MB or    │      │
│  │              │  │ Redis: shared    │      │
│  └──────────────┘  └──────────────────┘      │
└──────────────────────────────────────────────┘
```

### Setup

```bash
pip install mirage-ai
```

```python
from mirage import Workspace, MountMode
from mirage.resource.gcs import GCSResource
from mirage.resource.ram import RAMResource

ws = Workspace({
    "/docs": GCSResource(bucket="my-bucket", prefix="docs/"),
    "/scratch": RAMResource(),
})

# Agent just uses bash
result = await ws.execute('cat /docs/q1-revenue-projections.md')
print(await result.stdout_str())

result = await ws.execute('grep -r "compliance deadline" /docs/')
print(await result.stdout_str())

await ws.execute('echo "New meeting notes" > /docs/meeting-2026-05.md')
```

Or via CLI:

```yaml
# workspace.yaml
mode: WRITE
mounts:
  /docs:
    resource: gcs
    mode: WRITE
    config:
      bucket: my-bucket
      prefix: docs/
  /scratch:
    resource: ram
    mode: WRITE
```

```bash
mirage workspace create workspace.yaml --id bench
mirage execute -w bench -c 'ls /docs/'
mirage execute -w bench -c 'cat /docs/q1-revenue-projections.md'
mirage execute -w bench -c 'grep -r "compliance" /docs/'
```

### Agent Framework Integration

Works out of the box with:

| Framework | Integration |
|-----------|------------|
| OpenAI Agents SDK | `MirageSandboxClient` — agents run bash in the workspace |
| Vercel AI SDK | `mirageTools(ws)` — typed tool set for any model |
| LangChain | Workspace as tool layer |
| Claude Code / Cursor | CLI tool — `mirage execute -w <id> -c '<command>'` |
| Pydantic AI | Tool integration |
| OpenHands | Sandbox surface |

### Caching

Two-layer, configurable:

| Layer | What | Default | Redis option |
|-------|------|---------|-------------|
| Index cache | Metadata, listings | 10-min TTL | Shared across workers |
| File cache | Object bytes | 512 MB RAM | Shared across machines |

Redis backend enables multi-machine cache sharing — something our system doesn't have.

### What It Does Well

- **Breadth**: 15+ backends behind one interface. Agent writes `grep -r "release" /slack /github /linear` and gets results from three services in one command.
- **Zero new vocabulary**: LLMs already know bash. No new tool schemas to learn.
- **Portable workspaces**: Snapshot, clone, version. Move agent runs between machines.
- **Embeddable**: Library, not a service. Runs in-process inside FastAPI, Express, or any runtime.
- **Framework integrations**: Direct support for the major agent SDKs.

### What It Doesn't Do

- **No ETag/generation validation**: File cache uses TTL. If a file changes between TTL refreshes, the agent sees stale data. There's no per-read freshness check.
- **No write conflict detection**: No CAS, no optimistic concurrency. Last write wins.
- **No audit logging**: No per-operation log with agent identity, timestamps, or conflict records.
- **No FUSE required**: The shell is in-process (good), but agents can't use native Cursor tools (Read, Write, Grep) against it — they must use `ws.execute()` or the CLI.
- **Very new**: v0.0.1 released May 2026. API may change.

### How You'd Use It Instead of This Project

Replace the CLI + skill with a Mirage workspace. The agent's skill would say "use `mirage execute`" instead of `gcs-bench`:

```bash
mirage execute -w bench -c 'cat /docs/q1-revenue-projections.md'
mirage execute -w bench -c 'grep -r "compliance deadline" /docs/'
mirage execute -w bench -c 'echo "New content" >> /docs/meeting-notes.md'
```

You'd lose: ETag validation, CAS, audit logging.
You'd gain: multi-backend, portable workspaces, framework integrations.

---

## 2. Cloud Storage FUSE (gcsfuse)

### What It Is

Google's official open-source FUSE adapter that mounts a GCS bucket as a local directory. Any application — including AI agents using Cursor's native Read/Write/Grep tools — can access GCS objects as regular files without knowing they're on cloud storage.

- **Maintained by**: Google Cloud (official)
- **Language**: Go
- **Maturity**: Production-grade (used in GKE for AI/ML workloads)
- **Platforms**: macOS and Linux
- **Link**: https://github.com/GoogleCloudPlatform/gcsfuse

### How It Works

```
┌──────────────────────────────────┐
│         Cursor Agent             │
│                                  │
│  Read("/mnt/docs/q1-revenue.md") │
│  Grep("compliance", "/mnt/docs") │
│  Write("/mnt/docs/notes.md")     │
│                                  │
└──────────┬───────────────────────┘
           │ POSIX file operations
           │
┌──────────▼───────────────────────┐
│         gcsfuse (FUSE daemon)    │
│                                  │
│  ┌─────────────────────────┐     │
│  │ Stat Cache              │     │
│  │ (file metadata in RAM)  │     │
│  │ TTL: configurable       │     │
│  └─────────────────────────┘     │
│                                  │
│  ┌─────────────────────────┐     │
│  │ File Cache              │     │
│  │ (object bytes on SSD)   │     │
│  │ Size: configurable      │     │
│  └─────────────────────────┘     │
│                                  │
│  ┌─────────────────────────┐     │
│  │ Kernel List Cache       │     │
│  │ (directory listings)    │     │
│  │ TTL: configurable       │     │
│  └─────────────────────────┘     │
│                                  │
└──────────┬───────────────────────┘
           │ GCS REST API / gRPC
           │
┌──────────▼───────────────────────┐
│   Google Cloud Storage           │
│   (AES-256 encrypted at rest)    │
└──────────────────────────────────┘
```

### Setup (macOS)

```bash
# Install
brew install gcsfuse

# Create mount point
mkdir -p /tmp/gcs-docs

# Mount the bucket (scoped to a prefix)
gcsfuse \
  --only-dir bench \
  --stat-cache-capacity 20000 \
  --stat-cache-ttl 60s \
  --type-cache-ttl 60s \
  --file-cache-max-size-mb 1024 \
  --cache-dir /tmp/gcsfuse-cache \
  --implicit-dirs \
  glide-shared-fs /tmp/gcs-docs

# Now use it like local files
ls /tmp/gcs-docs/
cat /tmp/gcs-docs/q1-revenue-projections.md
grep -r "compliance deadline" /tmp/gcs-docs/
```

### Agent Integration

The beauty of gcsfuse: **agents don't need a skill at all**. They use native Cursor tools:

```
# Agent uses Read tool
Read("/tmp/gcs-docs/q1-revenue-projections.md")

# Agent uses Grep tool
Grep("compliance deadline", path="/tmp/gcs-docs/")

# Agent uses Write tool
Write("/tmp/gcs-docs/meeting-notes.md", "New content")
```

No CLI binary, no daemon, no Unix socket, no skill file. The local-docs skill from our project would work as-is — just point it at the mount directory instead of `docs/`.

### Performance Characteristics

| Operation | First access | Cached access | Notes |
|-----------|-------------|---------------|-------|
| stat (metadata) | ~30-50ms | <1ms (stat cache) | Cache TTL configurable |
| ls (directory) | ~50-100ms | <1ms (kernel cache) | Needs `--kernel-list-cache-ttl-secs` |
| read (small file) | ~50-100ms | <1ms (file cache) | After first read, served from local SSD/RAM |
| read (large file) | ~100ms+ | <1ms (file cache) | Parallel downloads available |
| write | ~100-200ms | N/A (write-through) | Flushed to GCS on close |
| rename | ~200ms | N/A | Two API calls (copy + delete) |

### Caching Options

```bash
# Metadata cache (stat calls)
--stat-cache-capacity 20000        # number of entries
--stat-cache-ttl 60s               # how long to trust cached metadata

# Directory listing cache
--kernel-list-cache-ttl-secs 60    # cache ls results in kernel

# File content cache
--file-cache-max-size-mb 4096      # max cache on disk (4GB)
--cache-dir /tmp/gcsfuse-cache     # where to store cached files

# For read-only workloads, mount as read-only for better caching
--o ro
```

### What It Does Well

- **Zero agent code**: No CLI, no skill, no daemon. Mount and go.
- **Native file tools**: Agents use Read/Write/Grep/Glob without knowing about GCS.
- **Production-grade**: Used by Google internally, maintained by the Cloud Storage team.
- **File cache**: After first read, sub-millisecond from local SSD.
- **Metadata prefetch**: Can preload file metadata on mount for faster first access.
- **Streaming writes**: v3.0+ uploads data as it's written, reducing latency.
- **GKE integration**: Automated profiles for AI/ML workloads.

### What It Doesn't Do

- **No write conflict detection**: Two machines can write to the same object. Last write wins silently. From the gcsfuse docs: "The default behavior is appropriate when the bucket is modified only via a single Cloud Storage FUSE mount. If multiple actors will be modifying a bucket, be sure to read the rest of this section carefully."
- **No audit logging**: No per-operation log. (GCS has access logs at the bucket level, but they're API-level, not agent-level.)
- **No freshness guarantee**: Cache uses TTL. Within the TTL window, stale data may be served. You can set TTL to 0 but then every read hits the API.
- **Not great for small random reads**: Optimized for sequential, large-file access (AI model weights, datasets). Small markdown files work fine but it's not the target use case.
- **Write-on-close semantics**: Writes are buffered locally and flushed to GCS when the file is closed. If the process crashes before close, the write is lost.
- **Multi-writer warning**: The official docs explicitly warn against multi-writer scenarios.

### How You'd Use It Instead of This Project

```bash
# One-time setup
brew install gcsfuse
mkdir -p /tmp/bench-docs
gcsfuse --only-dir bench --file-cache-max-size-mb 1024 \
  --cache-dir /tmp/gcsfuse-cache \
  glide-shared-fs /tmp/bench-docs

# Modify the local-docs skill to point at the mount
# skills/local-docs/SKILL.md → change "docs/" to "/tmp/bench-docs/"
```

That's it. The agent reads and writes as if the files are local. After first access, everything is cached. Encryption is handled by GCS (AES-256 at rest, TLS in transit).

---

## Head-to-Head Comparison

| Dimension | Mirage | gcsfuse | Our system |
|-----------|--------|---------|------------|
| **Setup time** | ~10 min (pip install + config) | ~5 min (brew install + mount) | ~30 min (build + auth + daemon) |
| **Agent integration** | `mirage execute` CLI or SDK | Native file tools (zero config) | CLI + skill file |
| **Backend coverage** | 15+ (S3, GDrive, GCS, Slack...) | GCS only | Drive + GCS |
| **Cache architecture** | Index + file (TTL-based) | Stat + kernel + file (TTL-based) | Index + file (ETag validated) |
| **Freshness on read** | TTL (stale risk) | TTL (stale risk) | Validated every read |
| **Write conflict detection** | None | None | CAS (generation/modifiedTime) |
| **Audit logging** | None | None (GCS access logs exist) | Per-operation JSONL |
| **Multi-machine cache** | Redis (shared) | Per-machine only | Per-machine only |
| **Multi-writer safety** | Unsafe | Explicitly warned against | Safe (CAS) |
| **Framework integrations** | OpenAI, Vercel, LangChain, etc. | Any (transparent FS) | Cursor skills only |
| **Portability** | Python 3.12+ or Node 20+ | macOS or Linux (FUSE) | Any OS (Go binary) |
| **Maturity** | v0.0.1 (May 2026) | Production (Google-maintained) | Prototype |

## Decision Guide

**Choose gcsfuse if**:
- Your agents only read documents (or one process writes)
- You want zero code and zero skills — mount and go
- You're okay with TTL-based freshness
- You're on macOS or Linux
- You want the simplest possible setup

**Choose Mirage if**:
- You need multi-backend (GCS + Slack + GitHub + Linear in one tree)
- You're building agents with OpenAI/Vercel/LangChain SDKs
- You want portable, snapshot-able workspaces
- You're okay with TTL-based freshness and no write safety

**Choose our system if**:
- Multiple agents write to the same files
- You need per-read freshness guarantees (medical, financial, compliance)
- You need an audit trail of which agent did what
- You need write conflict detection
- You want a single binary with no Python/Node runtime dependency
