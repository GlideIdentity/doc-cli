# gcs-bench

Fast, secure document CLI for AI agents backed by Google Cloud Storage.

## Install

### Homebrew (recommended)

```bash
brew tap glideidentity/tap https://github.com/GlideIdentity/doc-cli.git
brew install glideidentity/tap/gcs-bench
```

### Pre-built binary

Download from the [latest release](https://github.com/GlideIdentity/doc-cli/releases) and place in your `$PATH`:

```bash
curl -Lo gcs-bench https://github.com/GlideIdentity/doc-cli/releases/download/v0.1.0/gcs-bench-darwin-arm64
chmod +x gcs-bench
sudo mv gcs-bench /usr/local/bin/
```

### Build from source

```bash
git clone https://github.com/GlideIdentity/doc-cli.git
cd doc-cli
go build -o gcs-bench ./cmd/gcs-bench/
sudo mv gcs-bench /usr/local/bin/
```

## Setup

Run once after install:

```bash
gcs-bench setup --bucket glide-shared-fs --prefix YOUR_NAME
```

You need GCP credentials — either `gcloud auth application-default login` or a service account key at `~/.config/gcs-bench/sa-key.json`.

## Usage

```bash
gcs-bench find   --name "revenue"
gcs-bench read   --key "q1-revenue-projections.md"
gcs-bench search --query "compliance deadline"
gcs-bench create --name "notes.md" --body "Meeting notes for today"
gcs-bench update --key "notes.md" --body "Updated content" --expect-gen 12345
```

The daemon starts automatically on first command — no manual steps.

## Safe Updates

Every file has a generation number. Pass `--expect-gen` when updating to prevent conflicts:

```bash
# Read returns the current generation
gcs-bench read --key "report.md"
# => generation: 98765

# Update only succeeds if generation matches
gcs-bench update --key "report.md" --body "New content" --expect-gen 98765
```

If another agent wrote to the file since you last read it, the update fails safely.

## Admin

Admins can manage users. Each user gets an isolated GCS prefix with IAM enforcement.

```bash
gcs-bench admin add-user    --user alice --prefix team-alpha --bucket glide-shared-fs
gcs-bench admin list-users
gcs-bench admin remove-user --user alice
```

Requires `gcloud` CLI and IAM permissions (`roles/iam.serviceAccountAdmin` + `roles/storage.admin`).

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
