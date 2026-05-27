# GCS Encrypted File System CLI (`gcs-bench`)

Manage files in the `glide-shared-fs` GCS bucket. The daemon auto-starts on first use.

## Commands

**Find files by name:**
```bash
gcs-bench find --name "quarterly report"
```

**Read file content** (key includes your prefix):
```bash
gcs-bench read --key "bench/q1-revenue-projections.md"
```

**Full-text search:**
```bash
gcs-bench search --query "compliance deadline"
```

**Create a file** (name only — prefix is added automatically):
```bash
gcs-bench create --name "meeting-notes.md" --body "The content here"
```

**Update a file (appends by default, with CAS):**
```bash
gcs-bench update --key "bench/file.md" --body "New content" --expect-gen 12345
```

## Key Format

- `find` and `search` return keys with your prefix: `bench/filename.md`
- `read` and `update` require the full key (with prefix)
- `create` takes just the filename — the prefix is prepended automatically

## Output

All commands return JSON with `_elapsed_ms` timing and `generation` for concurrency control.

## Safe Update Workflow

1. `find` to get the file key and `generation`
2. `read` to get current content (also returns `generation`)
3. `update` with `--expect-gen` set to the generation from step 2

If another user modified the file, the update returns `"status": "conflict"`. Re-read and retry.

## Permissions

You can only access files in your configured prefix. Shared prefixes are read-only.
Trying to access other users' files returns `"status": "denied"`.

## Audit Log

All operations logged to `~/.config/gcs-bench/audit.jsonl`. Agent identity is auto-detected from config.
