# Off-the-Shelf Alternatives

Before shipping, here's what already exists and how each compares. Organized from most relevant to least.

---

## Tier 1: Direct Competitors (same problem space)

### Mirage (strukto-ai/mirage)

**What it is**: A unified virtual file system that mounts S3, Google Drive, GCS, Slack, Gmail, Redis, MongoDB, and more as a single filesystem tree. Agents use Unix-like tools (ls, cat, grep) to work across all backends.

**Key features**:
- Two-layer cache: index cache (metadata/listings) + file cache (object bytes) — same architecture we built
- RAM or Redis cache backends (Redis for multi-worker/multi-machine sharing)
- Python and TypeScript SDKs to embed in agent frameworks
- Drops into LangChain, CrewAI, AutoGen as a tool layer
- Any LLM that knows bash can use it — no new vocabulary

**How it compares**:

| Aspect | Mirage | Our system |
|--------|--------|------------|
| Backend coverage | 15+ (S3, GDrive, GCS, Slack, etc.) | 2 (Drive, GCS) |
| Cache architecture | Two-layer (index + file), configurable TTL | Three-layer (index + cache + ETag validation) |
| Freshness guarantee | TTL-based (stale risk) | ETag/generation validation (always correct) |
| Write conflict detection | Not documented | Optimistic concurrency (CAS) |
| Audit logging | Not documented | Full JSONL audit trail |
| Agent integration | SDK (Python/TS) | CLI + Cursor skill |
| Multi-machine cache | Redis backend | Per-machine only |

**Verdict**: Mirage is the closest off-the-shelf alternative. It has broader backend support and SDK integration. We have stronger freshness guarantees, write safety, and audit logging. If you don't need CAS or audit trails, **Mirage could replace this project**.

**Link**: https://github.com/strukto-ai/mirage

---

### Nexus (nexi-lab/nexus)

**What it is**: A virtual filesystem server with universal connectors, ReBAC permissions (Zanzibar-style), semantic search, skills lifecycle management, and a control panel.

**Key features**:
- Universal connectors: local FS, S3, GCS, Gmail, Google Drive, GitHub, Linear, Notion
- Zanzibar-style permissions (ReBAC) with multi-tenant isolation
- Semantic search + LLM document reading
- Content deduplication + versioning
- CLI, MCP Server, Python SDK, and REST API access
- Workspaces, memory, workflows, skills system

**How it compares**:

| Aspect | Nexus | Our system |
|--------|-------|------------|
| Scope | Full agent platform (files + memory + workflows) | File operations only |
| Permissions | ReBAC (enterprise-grade) | IAM-scoped SA key |
| Search | Semantic (vector) + full-text | Full-text only (index-based) |
| Complexity | Heavy (control panel, FUSE, RAG pipelines) | Minimal (single binary) |
| Setup time | Hours (many components) | Minutes (one binary + env vars) |
| Latency | Unknown (adds abstraction layers) | Measured, optimized |

**Verdict**: Nexus is far more ambitious — it's a full agent platform, not just a file tool. If you're building a complex multi-agent system with permissions, memory, and workflows, it's worth evaluating. For a fast document CLI, it's massive overkill.

**Link**: https://github.com/nexi-lab/nexus

---

## Tier 2: MCP Servers (same protocol, different approach)

### Google Drive Official MCP Server

**What it is**: Google's own remote MCP server at `https://drivemcp.googleapis.com/mcp/v1`. Works with Gemini CLI, Claude, and IDE clients.

**Available tools**: `create_file`, `download_file_content`, `get_file_metadata`, `get_file_permissions`, `list_recent_files`, `read_file_content`, `search_files` (7 tools)

**How it compares**:

| Aspect | Official MCP | Our system |
|--------|-------------|------------|
| Setup | Config in MCP client settings | Build + authenticate |
| Caching | None (every call hits Drive API) | Daemon with index + cache |
| Latency | ~500-2000ms per call (MCP + Drive) | 0-13ms (cached), ~500ms (validated) |
| CAS / conflict detection | No | Yes |
| Audit logging | No | Yes |
| Cursor agent skill | 46 tools loaded into context | 1 lean skill file |
| Token cost | High (large tool schemas in context) | Low (minimal skill) |

**Verdict**: The official MCP server is the zero-config option. Plug it into your MCP client and go. But it's 10-100x slower than our daemon, has no caching, no conflict detection, and loads 46 tool schemas into agent context. **This is what we benchmarked against — and what motivated building this project.**

**Link**: https://developers.google.com/workspace/drive/api/guides/configure-mcp-server

### Community Drive MCP Servers

- **lsmc-bio/gdrive-mcp** — Full Google Workspace (Drive, Docs, Sheets, Slides, Gmail, Calendar, Apps Script). Deep integration but heavy.
- **S-V6/google-drive-mcp** — Drive, Docs, Sheets, Slides, Calendar. Multi-format, shared drives support. Good breadth, no caching.

Same latency and token cost issues as the official MCP server — every call is a live API hit with full MCP protocol overhead.

---

## Tier 3: FUSE Mount Solutions (different architecture)

### Cloud Storage FUSE (gcsfuse)

**What it is**: Google's official FUSE adapter that mounts GCS buckets as local directories. Agents use standard file tools (Read/Write/Grep) without knowing it's cloud storage.

**Key features**:
- File cache (local SSD or RAM) for repeated reads
- Metadata prefetch on mount
- Parallel downloads for large files
- GKE profiles for AI/ML workloads (training, serving, checkpointing)
- Anywhere Cache for same-zone SSD caching (70-96% latency reduction)

**How it compares**:

| Aspect | gcsfuse | Our system |
|--------|---------|------------|
| Agent integration | Transparent (local file tools) | CLI + skill |
| First read latency | ~50-100ms (GCS API) | ~50-80ms (GCS API) |
| Cached read latency | <1ms (local SSD/RAM cache) | 0ms (in-memory) + validation |
| Write latency | ~100-200ms (write-through) | ~100ms (API) |
| Freshness | TTL-based (`--dir-cache-time`) | ETag/generation validation |
| CAS | No | Yes (generation-based) |
| Audit logging | No (use GCS access logs) | Yes (per-operation) |
| Setup | `gcsfuse bucket /mnt/point` | Build binary + env vars |
| Multi-machine writes | Unsafe (no conflict detection) | Safe (CAS) |
| Platform | Linux only (FUSE) | Any (Go binary) |

**Verdict**: gcsfuse is the simplest option — mount and go. For read-heavy, single-writer workloads, it's excellent. The agent doesn't need a skill, it just reads local files. But it has no write conflict detection, runs only on Linux, and TTL-based freshness can serve stale data. **If your agents only read and one process writes, gcsfuse is the answer. If multiple agents write, it's not safe.**

### rclone mount

**What it is**: Open-source tool that mounts 70+ cloud storage backends as local directories. VFS cache with configurable modes.

**Key features**:
- Supports Google Drive, GCS, S3, Dropbox, OneDrive, etc.
- VFS cache modes: off, minimal, writes, full
- Configurable cache size, TTL, read-ahead
- Works on macOS, Linux, Windows

**How it compares**:

| Aspect | rclone mount | Our system |
|--------|-------------|------------|
| Backend coverage | 70+ | 2 |
| Cache modes | 4 (off/minimal/writes/full) | Validated + index |
| Platform | macOS, Linux, Windows | Any (Go binary) |
| Write safety | None (last write wins) | CAS |
| Agent integration | Transparent (local file tools) | CLI + skill |
| Audit | No | Yes |

**Verdict**: rclone is the Swiss army knife. If you need to mount any cloud storage as a local directory, it's unbeatable for breadth. Same write-safety gap as gcsfuse — no conflict detection.

---

## Tier 4: Agent Security Sandboxes (different problem)

### AgentFense

Four-level filesystem permissions for AI agents: none (invisible), view (list only), read, write. Docker and bubblewrap isolation. Copy-on-write for multi-sandbox safety.

Not a cloud storage tool — it's a local sandbox. Relevant if you need to restrict which local files an agent can access, but doesn't solve remote storage access.

---

## Decision Matrix

| Need | Best off-the-shelf option | Our system advantage |
|------|--------------------------|---------------------|
| Fast cached reads from cloud storage | **Mirage** or **gcsfuse** | ETag-validated freshness |
| Multi-backend (S3 + Drive + GCS) | **Mirage** or **rclone** | N/A (we only do Drive + GCS) |
| Safe multi-user writes | **None** (gap in market) | CAS with generation/modifiedTime |
| Audit trail per agent operation | **None** (gap in market) | Full JSONL audit logging |
| Zero-config agent integration | **Google Drive Official MCP** | N/A (requires setup) |
| Transparent file access (no skill) | **gcsfuse** or **rclone mount** | N/A (requires CLI + skill) |
| Enterprise permissions | **Nexus** | N/A (SA-level only) |
| Minimal dependencies | Our system | Single Go binary |

## Recommendation

**If you only need fast reads and don't care about write safety**: Use **gcsfuse** (GCS) or **rclone mount** (any backend). Zero code to write. Agents use native file tools.

**If you need multi-backend support with caching**: Use **Mirage**. It has the broadest coverage and a similar cache architecture. Add your own CAS layer if write safety matters.

**If you need write conflict detection + audit logging**: **Ship this project**. No off-the-shelf tool provides optimistic concurrency control with per-operation audit logging for AI agent workflows. This is a genuine gap in the market.

**If you need enterprise permissions and semantic search**: Evaluate **Nexus**, but expect significant setup complexity.
