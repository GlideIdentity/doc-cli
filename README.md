# Document CLI for AI Agents

Fast, secure, multi-user document access for AI agents. Two backends: Google Drive and Google Cloud Storage.

## Why

AI agents need to read and write documents. The standard approaches (MCP servers, FUSE mounts, local file sync) either lack write safety, have no audit trail, or serve stale data. This project provides:

- **Sub-millisecond cached reads** with per-read freshness validation
- **Optimistic concurrency control** — prevents lost updates when multiple agents write
- **Per-user folder isolation** — enforced by GCS IAM, not just the CLI
- **Full audit trail** — every operation logged with agent ID, machine, timestamps
- **Single binary** — no runtime dependencies

## Quick Start (GCS — recommended)

```bash
# Download binary (macOS ARM)
curl -L -o gcs-bench https://github.com/eranhaggiag/clitest/releases/latest/download/gcs-bench-darwin-arm64
chmod +x gcs-bench

# First-time setup
./gcs-bench setup --bucket my-bucket --prefix my-team

# Use it — daemon auto-starts
./gcs-bench find --name "revenue"
./gcs-bench read --key "q1-report.md"
./gcs-bench search --query "compliance deadline"
./gcs-bench create --name "notes.md" --body "Meeting notes..."
./gcs-bench update --key "notes.md" --body "New section" --expect-gen 12345
```

## Multi-User Setup

```bash
# Admin adds users with isolated prefixes
./gcs-bench admin add-user --user alice --prefix team-alpha
./gcs-bench admin add-user --user bob --prefix team-beta

# Each user can only access their own prefix
# Alice: team-alpha/* (read/write) + shared/* (read-only)
# Bob:   team-beta/*  (read/write) + shared/* (read-only)
# Enforced by GCS IAM — can't bypass even with gsutil
```

## Build from Source

```bash
make build        # Build both CLIs for current platform
make build-all    # Cross-compile for macOS + Linux (arm64/amd64)
```

## Architecture

See [docs/architecture/](docs/architecture/) for detailed documentation:

1. [Overview](docs/architecture/01-overview.md) — system diagram, performance summary
2. [Daemon](docs/architecture/02-daemon-architecture.md) — connection pooling, auto-start
3. [Caching](docs/architecture/03-caching-and-freshness.md) — ETag validation, freshness
4. [Concurrency](docs/architecture/04-concurrency-and-safety.md) — CAS, conflict detection
5. [Shipping](docs/architecture/05-multi-user-shipping.md) — MVP to enterprise checklists
6. [Security](docs/architecture/06-security-model.md) — least privilege, encryption
7. [Benchmarks](docs/architecture/07-benchmark-results.md) — measured latency
8. [Alternatives](docs/architecture/08-alternatives.md) — Mirage, gcsfuse, MCP servers
9. [Deep Dive](docs/architecture/09-deep-dive-mirage-gcsfuse.md) — Mirage vs gcsfuse
10. [Permissions](docs/architecture/10-per-user-permissions.md) — IAM + CLI enforcement

## Performance

| Operation | Drive CLI (optimized) | GCS CLI (target) | Local Files |
|-----------|----------------------|------------------|-------------|
| Find      | 12ms                 | 0ms (index)      | <1ms        |
| Read      | ~500ms (validated)   | ~50ms (validated) | <1ms        |
| Search    | 13ms                 | 0ms (index)      | 3ms         |

## Two Backends

| | Google Drive (`gdrive-bench`) | GCS (`gcs-bench`) |
|---|---|---|
| Best for | Existing Drive users | New deployments, multi-user |
| Auth | OAuth2 (browser flow) | Service account key (no browser) |
| Concurrency | modifiedTime-based | Generation-based (true CAS) |
| Permissions | drive.file scope + folder | IAM Conditions + prefix |
| Speed | ~500ms validated reads | ~50ms validated reads |
