# Google Drive Benchmark: Custom Go CLI vs. Local Markdown Files

Benchmarks comparing agent performance when using a native Go CLI to access Google Drive vs. reading pre-synced local markdown files.

## Hypothesis

For common document operations (find, read, search, create, update), an agent working with pre-synced local markdown files is significantly faster, cheaper, and more reliable than an agent using a CLI — even a purpose-built native Go binary.

## Quick Start

### 1. Build the CLI

```bash
go build -o gdrive-bench ./cmd/gdrive-bench/
```

### 2. Authenticate

Place your Google OAuth2 `credentials.json` in `~/.config/gdrive-bench/` and run:

```bash
./gdrive-bench auth
```

Set the target folder:

```bash
export GDRIVE_FOLDER=your-folder-id
```

### 3. Run the raw benchmark

```bash
npx tsx scripts/raw-benchmark.ts
```

### 4. Run agent-driven benchmark

Follow the protocol in `benchmark/protocol.md` — run each task through both the `gdrive-cli` and `local-docs` skills.

### 5. Analyze results

```bash
npx tsx benchmark/analyze.ts
```

## Project Structure

```
cmd/gdrive-bench/     Custom Go CLI (native binary, ~15ms startup)
docs/                 Pre-synced local markdown test documents
skills/
  gdrive-cli/         Cursor skill for CLI-based operations
  local-docs/         Cursor skill for local file operations
benchmark/
  protocol.md         5-task benchmark protocol
  results/            JSON result files
  analyze.ts          Results analysis script
scripts/
  raw-benchmark.ts    Script-driven latency benchmark
```

## Metrics

- **Latency**: wall-clock time per task
- **Token usage**: input + output tokens per task
- **Tool calls**: number of tool invocations per task
- **Success rate**: pass/fail per task
- **CLI self-timing**: `_elapsed_ms` from the Go binary (API latency only)
