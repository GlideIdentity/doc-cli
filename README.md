# gcs-bench

Fast, secure document CLI for AI agents backed by Google Cloud Storage.

- **0ms** find/search via local index
- **~50ms** reads with generation-validated cache
- **Optimistic concurrency** (CAS) prevents lost updates
- **Per-user prefix isolation** with IAM enforcement
- **Audit logging** of every operation (JSONL)

## Install

### Homebrew (recommended)

```bash
brew tap glideidentity/tap
brew install glideidentity/tap/gcs-bench
```

### Build from source

Requires Go 1.21+:

```bash
git clone https://github.com/GlideIdentity/doc-cli.git
cd doc-cli
go build -o gcs-bench ./cmd/gcs-bench/
sudo mv gcs-bench /usr/local/bin/
```

### Cross-compile

```bash
make build-all   # builds darwin-arm64, darwin-amd64, linux-amd64, linux-arm64
```

Binaries land in `dist/`.

## Prerequisites

- `**gcloud` CLI** (for admin commands and initial auth)
- **GCP project**: `shared-file-system` (GlideIdentity org)
- **Bucket**: `glide-shared-fs`

## Setup

Run once after install:

```bash
gcs-bench setup --bucket glide-shared-fs --prefix YOUR_NAME
```

You need a service account key at `~/.config/gcs-bench/sa-key.json`. Ask an admin to onboard you:

```bash
# Admin runs this to create your account
gcs-bench admin add-user --user YOUR_NAME --prefix YOUR_NAME --bucket glide-shared-fs
```

Or use Application Default Credentials:

```bash
gcloud auth application-default login
```

## Usage

```bash
gcs-bench find   --name "revenue"
gcs-bench read   --key "YOUR_PREFIX/q1-revenue-projections.md"
gcs-bench search --query "compliance deadline"
gcs-bench create --name "notes.md" --body "Meeting notes for today"
gcs-bench update --key "YOUR_PREFIX/notes.md" --body "Updated content" --expect-gen 12345
```

The daemon starts automatically on first command — no manual steps.

**Keys include your prefix.** When you `find` or `search`, results return full keys like `YOUR_PREFIX/filename.md`. Use those keys for `read` and `update`.

## Safe Updates

Every file has a generation number. Pass `--expect-gen` when updating to prevent conflicts:

```bash
# Read returns the current generation
gcs-bench read --key "bench/report.md"
# => generation: 98765

# Update only succeeds if generation matches
gcs-bench update --key "bench/report.md" --body "New content" --expect-gen 98765
```

If another agent wrote to the file since you last read it, the update fails safely with `"status": "conflict"`.

## Admin

Admins can manage users. Each user gets an isolated GCS prefix with IAM enforcement.

```bash
gcs-bench admin add-user    --user alice --prefix team-alpha --bucket glide-shared-fs
gcs-bench admin list-users
gcs-bench admin remove-user --user alice
```

Requires `gcloud` CLI and IAM permissions on the `shared-file-system` project (`roles/iam.serviceAccountAdmin` + `roles/storage.admin`).

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

Environment variable overrides: `GCS_BUCKET`, `GCS_PREFIX`, `GCS_AGENT_ID`.

## Audit Log

All operations are logged to `~/.config/gcs-bench/audit.jsonl`:

```json
{"timestamp":"2026-05-27T06:30:00Z","agent_id":"erik@macbook","operation":"read","key":"erik/revenue.md","result":"ok","latency_ms":45}
```

## Architecture

See `[docs/architecture/](docs/architecture/)` for detailed documentation:

1. [Overview & system diagram](docs/architecture/01-overview.md)
2. [Daemon architecture](docs/architecture/02-daemon-architecture.md)
3. [Caching & freshness](docs/architecture/03-caching-and-freshness.md)
4. [Concurrency & safety](docs/architecture/04-concurrency-and-safety.md)
5. [Security model](docs/architecture/06-security-model.md)
6. [Benchmark results](docs/architecture/07-benchmark-results.md)
7. [Alternatives (Mirage, gcsfuse)](docs/architecture/08-alternatives.md)
8. [Per-user permissions](docs/architecture/10-per-user-permissions.md)

## GCP Project


| Resource   | Value                                                         |
| ---------- | ------------------------------------------------------------- |
| Project    | `shared-file-system`                                          |
| Org        | GlideIdentity                                                 |
| Bucket     | `glide-shared-fs`                                             |
| Region     | `me-west1`                                                    |
| SA pattern | `gcs-bench-{user}@shared-file-system.iam.gserviceaccount.com` |


