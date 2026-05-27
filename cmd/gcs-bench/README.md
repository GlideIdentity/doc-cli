# gcs-bench — GCS Encrypted File System CLI

Fast, secure, multi-user document CLI for AI agents backed by Google Cloud Storage.

## Features

- **Sub-millisecond cached reads** with ETag/generation-based freshness validation
- **Optimistic concurrency control** (CAS) prevents lost updates
- **Per-user folder isolation** via GCS IAM Conditions (server-enforced, can't bypass)
- **Audit logging** — every operation logged with agent ID, machine, timestamps
- **Auto-start daemon** — connection pooling, local index, content cache
- **Single binary** — no runtime dependencies, cross-compiled for macOS and Linux

## Quick Start

```bash
# First-time setup
gcs-bench setup --bucket my-bucket --prefix my-team

# That's it. Commands auto-start the daemon.
gcs-bench find --name "revenue"
gcs-bench read --key "q1-revenue-projections.md"
gcs-bench search --query "compliance deadline"
gcs-bench create --name "notes.md" --body "Meeting notes..."
gcs-bench update --key "notes.md" --body "New section" --expect-gen 12345
```

## Multi-User Setup (Admin)

```bash
# Add a user with isolated prefix
gcs-bench admin add-user --user alice --prefix team-alpha --bucket my-bucket

# List configured users
gcs-bench admin list-users

# Remove a user (deletes SA, revokes all access instantly)
gcs-bench admin remove-user --user alice
```

## How Permissions Work

Each user gets a GCS service account restricted to their prefix:

- Alice (team-alpha/): can read/write team-alpha/*, read shared/*
- Bob (team-beta/): can read/write team-beta/*, read shared/*
- Neither can see the other's files — enforced by GCS IAM, not just the CLI

## Config

`~/.config/gcs-bench/config.json`:

```json
{
  "bucket": "my-bucket",
  "user_prefix": "team-alpha",
  "shared_prefixes": ["shared"],
  "agent_id": "alice@macbook",
  "credentials": "~/.config/gcs-bench/sa-key.json"
}
```

Environment variables override config: `GCS_BUCKET`, `GCS_PREFIX`, `GCS_AGENT_ID`.

## Daemon

The daemon auto-starts on first command. To manage manually:

```bash
gcs-bench daemon start    # pre-warms connections, builds index
gcs-bench daemon status   # check if running
gcs-bench daemon stop     # graceful shutdown
```

## Audit Log

`~/.config/gcs-bench/audit.jsonl` — one JSON line per operation:

```json
{"timestamp":"2026-05-27T06:30:00Z","agent_id":"alice@macbook","machine":"alice-mbp","user_prefix":"team-alpha","operation":"read","key":"team-alpha/revenue.md","result":"ok","latency_ms":45}
```
