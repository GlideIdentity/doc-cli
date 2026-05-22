# Google Drive CLI (`gdrive-bench`)

Manage Google Drive files using the `gdrive-bench` CLI. All operations are scoped to the folder in `$GDRIVE_FOLDER`.

**Start the daemon first** for faster operations (warm connection pool):
```bash
gdrive-bench daemon start
```

## Commands

**Find files by name:**
```bash
gdrive-bench find --name "quarterly report"
```

**Read file content** (use `--mime google-doc` to skip a metadata round-trip):
```bash
gdrive-bench read --id FILE_ID --mime google-doc
```

**Full-text search:**
```bash
gdrive-bench search --query "compliance deadline"
```

**Create a document:**
```bash
gdrive-bench create --name "Meeting Notes" --body "The content here"
```

**Update a document (appends by default):**
```bash
gdrive-bench update --id FILE_ID --body "New section content"
```

## Output

All commands return JSON with `_elapsed_ms` timing. Example:
```json
{"files": [{"id": "abc123", "name": "Q1 Report"}], "_elapsed_ms": 142}
```

## Safe Update Workflow

To update a file safely in a multi-user environment:

1. `find` to get the file ID and `modifiedTime`
2. `read` to get current content
3. `update` with `--expect-modified` set to the `modifiedTime` from step 1

If another user edited the file between your read and write, the update returns `"status": "conflict"` instead of writing. Re-read and retry.

```bash
gdrive-bench update --id FILE_ID --body "New content" --expect-modified "2026-05-22T19:01:20.778Z"
```

## Audit Log

All operations are logged to `~/.config/gdrive-bench/audit.jsonl` with agent ID, machine, timestamp, operation, result, and latency. Set `GDRIVE_AGENT_ID` to identify your agent.
