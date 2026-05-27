# Document CLI for AI Agents

Fast, secure, multi-user document access for AI agents backed by Google Cloud Storage.

## Quick Start (30 seconds)

```bash
# 1. Clone and build
git clone https://github.com/GlideIdentity/doc-cli.git
cd doc-cli && git checkout initial-setup
go build -o gcs-bench ./cmd/gcs-bench/

# 2. Setup (one time)
./gcs-bench setup --bucket glide-shared-fs --prefix YOUR_NAME

# 3. Use it (daemon auto-starts, no manual steps)
./gcs-bench find --name "revenue"
./gcs-bench read --key "q1-revenue-projections.md"
./gcs-bench search --query "compliance deadline"
./gcs-bench create --name "test.md" --body "Hello from $(whoami)"
./gcs-bench update --key "test.md" --body "Added a section" --expect-gen 12345
```

That's it. The daemon starts automatically on first command.

## Prerequisites

- **Go 1.21+** — `brew install go` or https://go.dev/dl/
- **GCP credentials** — either:
  - `gcloud auth application-default login` (simplest), or
  - A service account key at `~/.config/gcs-bench/sa-key.json`

## Pre-built Binaries

If you don't want to build from source, grab a binary from `dist/`:

| Platform | Binary |
|----------|--------|
| macOS ARM (M1/M2/M3) | `dist/gcs-bench-darwin-arm64` |
| macOS Intel | `dist/gcs-bench-darwin-amd64` |
| Linux AMD64 | `dist/gcs-bench-linux-amd64` |
| Linux ARM64 | `dist/gcs-bench-linux-arm64` |

```bash
cp dist/gcs-bench-darwin-arm64 ./gcs-bench
chmod +x gcs-bench
./gcs-bench setup --bucket glide-shared-fs --prefix YOUR_NAME
```

## Admin: Managing Users

Admins can add, remove, and list users. Each user gets an isolated folder in GCS with their own service account — enforced by IAM (can't bypass even with gsutil).

```bash
# Add a user (creates GCS service account + IAM bindings + config)
./gcs-bench admin add-user --user alice --prefix team-alpha --bucket glide-shared-fs

# List all configured users
./gcs-bench admin list-users

# Remove a user (deletes SA, revokes all access instantly)
./gcs-bench admin remove-user --user alice
```

**Requirements to be an admin:**
- `gcloud` CLI installed and authenticated (`gcloud auth login`)
- IAM permissions on the GCP project: `iam.serviceAccounts.create`, `storage.buckets.setIamPolicy`
- Typically: `roles/iam.serviceAccountAdmin` + `roles/storage.admin` on the project

## What Each User Gets

| | Own prefix (e.g. `team-alpha/`) | Other users' prefixes | `shared/` |
|---|---|---|---|
| Read | Yes | **Denied** | Yes |
| Write | Yes | **Denied** | **Denied** |
| Delete | Yes | **Denied** | **Denied** |

Enforced at two layers:
1. **GCS IAM Conditions** — server-side, can't bypass
2. **CLI validation** — friendly error messages before hitting the API

## Config

Stored at `~/.config/gcs-bench/config.json`:

```json
{
  "bucket": "glide-shared-fs",
  "user_prefix": "your-name",
  "shared_prefixes": ["shared"],
  "agent_id": "you@your-machine",
  "credentials": "~/.config/gcs-bench/sa-key.json"
}
```

Environment variables override config: `GCS_BUCKET`, `GCS_PREFIX`, `GCS_AGENT_ID`.

## All Commands

```
gcs-bench setup    --bucket B --prefix P             First-time setup
gcs-bench admin    [add-user|remove-user|list-users]  Manage users (admin only)
gcs-bench daemon   [start|stop|status]                Manage daemon
gcs-bench find     --name "..."                       Find files by name
gcs-bench read     --key "file.md"                    Read file content
gcs-bench search   --query "..."                      Full-text search
gcs-bench create   --name "file.md" --body "..."      Create new file
gcs-bench update   --key "file.md" --body "..."       Update with conflict detection
```

## Build from Source

```bash
make build        # Build gcs-bench + gdrive-bench for current platform
make build-all    # Cross-compile for macOS + Linux (arm64/amd64)
```

## Architecture

See [docs/architecture/](docs/architecture/) for detailed documentation (10 files covering caching, concurrency, security, permissions, alternatives).

## Audit Log

Every operation is logged to `~/.config/gcs-bench/audit.jsonl`:

```json
{"timestamp":"2026-05-27T06:30:00Z","agent_id":"erik@macbook","machine":"eriks-mbp","user_prefix":"erik","operation":"read","key":"erik/revenue.md","result":"ok","latency_ms":45}
```
