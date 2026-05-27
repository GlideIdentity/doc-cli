# Concurrency and Safety

## The Multi-User Problem

When multiple agents on different machines work with the same documents:

1. **Stale reads** — agent reads old content, makes wrong decisions
2. **Lost updates** — two agents read, both modify, last write wins silently
3. **Split brain** — different machines have different versions
4. **No audit trail** — can't tell which agent did what

## How This System Addresses Each

### 1. Stale Reads → ETag Validation

Every Read checks the file's version before returning cached content. See [03-caching-and-freshness.md](03-caching-and-freshness.md).

### 2. Lost Updates → Optimistic Concurrency Control (CAS)

Updates use compare-and-swap semantics:

```
Agent reads file → gets version identifier
Agent modifies content locally
Agent sends update with --expect-modified (Drive) or --expect-gen (GCS)
Server checks: has the file changed since that version?
  ├── No  → Write succeeds
  └── Yes → Write rejected with "conflict" status
Agent must re-read and retry
```

#### Google Drive: modifiedTime-based

```bash
gdrive-bench update --id FILE_ID \
  --body "New section" \
  --expect-modified "2026-05-22T19:01:20.778Z"
```

If another user edited since that timestamp:
```json
{
  "status": "conflict",
  "error": "file was modified by another user since you last read it",
  "expected_modified": "2026-05-22T19:01:20.778Z",
  "actual_modified": "2026-05-22T19:25:59.081Z"
}
```

#### GCS: Generation-based (true CAS)

```bash
gcs-bench update --key "file.md" \
  --body "New section" \
  --expect-gen 12345
```

GCS provides stronger guarantees than Drive:
- Generation numbers are monotonically increasing integers
- `If-Generation-Match` is enforced server-side atomically
- No time-based race windows (unlike modifiedTime which has second granularity)

```go
condObj := obj.If(storage.Conditions{GenerationMatch: attrs.Generation})
writer := condObj.NewWriter(ctx)
// If generation changed between Attrs and Write, the write fails atomically
```

### 3. Split Brain → Authoritative Source

The API (Drive or GCS) is always the source of truth. Local caches and indexes are optimization layers, not authoritative stores. Writes always go through the API.

### 4. No Audit Trail → JSONL Audit Log

Every operation is logged to `~/.config/{gdrive,gcs}-bench/audit.jsonl`:

```json
{
  "timestamp": "2026-05-22T19:26:11.123Z",
  "agent_id": "agent-10007",
  "machine": "Erans-MacBook-Pro.local",
  "operation": "update",
  "file_id": "1zhnxBN...",
  "file_name": "Q1 2026 Revenue Projections",
  "source": "api",
  "result": "conflict",
  "detail": "expected=2026-01-01 actual=2026-05-22",
  "latency_ms": 659
}
```

Fields:
- `agent_id` — set via `GDRIVE_AGENT_ID` or `GCS_AGENT_ID` env var, defaults to PID
- `machine` — hostname, auto-detected
- `operation` — find, read, search, create, update
- `result` — ok, error, conflict
- `source` — api, index, cache-validated, cache-invalidated
- `latency_ms` — wall-clock time for the operation

## What Is NOT Protected

| Risk                      | Status          | Mitigation Path                   |
|---------------------------|-----------------|-----------------------------------|
| Stale reads               | Protected       | ETag/generation validation        |
| Lost updates              | Protected       | CAS on every write                |
| Stale search results      | Partial         | Index refreshed on daemon restart |
| Concurrent create (dupes) | Not protected   | Add name-uniqueness check         |
| File deletion conflicts   | Not protected   | Add generation check on delete    |
| Cross-daemon coordination | Not protected   | Add distributed lock service      |

## Safe Update Workflow for Agents

The CLI skill teaches agents this pattern:

```
1. find  → get file ID/key + version (modifiedTime or generation)
2. read  → get current content (also returns version)
3. update → with --expect-modified or --expect-gen set to version from step 2
4. if "conflict" → go back to step 2 and retry
```

This is optimistic — it assumes conflicts are rare. For high-contention scenarios (many agents editing the same file simultaneously), a pessimistic locking approach would be needed.
