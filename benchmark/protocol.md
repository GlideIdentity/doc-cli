# Benchmark Protocol: Google Drive CLI vs. Local Markdown Files

## Setup

- **CLI path**: Uses `gdrive-bench` binary via the `gdrive-cli` skill
- **Local path**: Uses native Cursor tools (Read, Write, Glob, Grep, StrReplace) via the `local-docs` skill
- **Test data**: 12 documents in `docs/` directory (also present in Google Drive under `$GDRIVE_FOLDER`)

## Tasks

Each task is performed 3 times per path. Record results after each execution.

### Task 1: Find by Name
> "Find the document about Q1 revenue projections"

**Success criteria**: Agent identifies `q1-revenue-projections.md` (or the Drive equivalent)

### Task 2: Read Content
> "Read the full content of the product roadmap document and summarize its key milestones"

**Success criteria**: Agent reads and accurately summarizes the content of `product-roadmap-2026.md`

### Task 3: Full-Text Search
> "Find all documents that mention 'compliance deadline'"

**Success criteria**: Agent finds the correct set of documents containing that phrase

### Task 4: Create Document
> "Create a new document called 'Meeting Notes May 2026' with the following content:
> - Attendees: Alice, Bob, Charlie
> - Topic: Q2 planning
> - Decision: Increase marketing budget by 15%
> - Action item: Bob to prepare revised budget by June 1"

**Success criteria**: New document created with correct name and complete content

### Task 5: Update Document
> "Add a new section called 'Budget Revisions' to the Q1 revenue projections document with the following: 'Based on Q1 actuals, the revenue target for Q2 has been revised upward to $4.2M, reflecting a 12% increase over the original forecast.'"

**Success criteria**: Existing document updated with new section appended

## Metrics to Record

For each task execution, record:

| Metric | How to Measure |
|--------|---------------|
| `task_id` | 1-5 |
| `path` | "cli" or "local" |
| `run` | 1, 2, or 3 |
| `start_time` | ISO timestamp when prompt is sent |
| `end_time` | ISO timestamp when agent finishes |
| `latency_ms` | end_time - start_time |
| `tool_calls` | Count of tool invocations |
| `success` | true/false |
| `cli_elapsed_ms` | `_elapsed_ms` from CLI output (CLI path only) |
| `notes` | Any errors, retries, or observations |

## Results File Format

Save each run to `benchmark/results/run-TIMESTAMP.json`:
```json
{
  "timestamp": "2026-05-22T15:00:00Z",
  "path": "cli",
  "results": [
    {
      "task_id": 1,
      "run": 1,
      "latency_ms": 3200,
      "tool_calls": 2,
      "success": true,
      "cli_elapsed_ms": 342,
      "notes": ""
    }
  ]
}
```
