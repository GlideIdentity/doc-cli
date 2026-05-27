# Benchmark Results

## Raw Latency (measured May 22, 2026)

### Optimization Journey — Google Drive CLI

| Iteration                          | Find     | Read     | Search   | Multi-user safe? |
|------------------------------------|----------|----------|----------|------------------|
| Original CLI (cold, no daemon)     | 1,933ms  | 6,604ms  | 1,199ms  | Yes              |
| + `--mime` skip (1 fewer API call) | 1,933ms  | 2,525ms  | 1,199ms  | Yes              |
| + Daemon (warm connection pool)    | 559ms    | 876ms    | 544ms    | Yes              |
| + Local index + content cache      | 0ms      | 0ms      | 0ms      | No               |
| + ETag validation (freshness)     | 0-12ms   | ~500ms   | 0-13ms   | Yes              |
| Local markdown files               | <1ms     | <1ms     | 3ms      | No               |

### Raw Benchmark Output (with daemon, 10 iterations)

| Operation | Path      | Avg (ms) | P50 (ms) | P95 (ms) | Min (ms) | Max (ms) |
|-----------|-----------|----------|----------|----------|----------|----------|
| find      | drive-cli | 12       | 11       | 16       | 9        | 16       |
| find      | local     | 0        | 0        | 0        | 0        | 0        |
| read      | drive-cli | 497      | 319      | 1,402    | 259      | 1,402    |
| read      | local     | 0        | 0        | 1        | 0        | 1        |
| search    | drive-cli | 13       | 12       | 17       | 11       | 17       |
| search    | local     | 3        | 3        | 4        | 1        | 4        |

### Where Time Goes (Drive Read breakdown)

```
Total read latency: ~500ms (validated cache hit)
├── CLI process startup:     ~5ms
├── Unix socket round-trip:  ~2ms
├── Daemon handler:          ~1ms
├── Drive Files.Get (meta):  ~490ms  ← 98% of time (network to Google)
├── Cache comparison:        ~0ms
└── JSON encoding:           ~0ms
```

The ~500ms floor is irreducible — it's the network round-trip to Google's Drive API servers for the metadata validation call. The only way below this is to skip validation (accepting stale risk) or move to a faster backend (GCS).

### Why GCS Is Faster (Expected)

| Layer                    | Drive        | GCS          | Why                                    |
|--------------------------|-------------|-------------|----------------------------------------|
| API protocol             | REST/JSON   | gRPC/proto  | Binary protocol, less parsing          |
| File model               | Google Doc  | Raw bytes   | No export/conversion needed            |
| Metadata call            | ~500ms      | ~30-50ms    | No collaboration features overhead     |
| Content download         | ~400ms      | ~30-50ms    | Direct object access, no export layer  |
| Auth                     | OAuth2 flow | SA JWT      | No browser, auto-refresh               |
| Concurrency primitive    | modifiedTime| generation  | Server-side atomic CAS vs time compare |

## Key Insights

### 1. The LLM is the bottleneck, not the file system

At the agent level, most wall-clock time is spent on model inference (~1-3s per step), not waiting for file operations. A 500ms read vs. a 1ms read adds ~500ms to a ~5s agent step — a 10% difference, not a 500x difference.

### 2. Every CLI optimization converges toward local caching

The fastest CLI we could build (daemon + index + cache) returned 0ms — because it was serving from local memory. At that point, it's functionally identical to local markdown files. The speed ceiling for any API-backed tool is "cache everything locally."

### 3. Freshness costs latency

The validated cache (multi-user safe) is ~500ms. The unvalidated cache (stale risk) is 0ms. That ~500ms is the price of correctness. For non-critical data, skip validation. For medical/financial data, always validate.

### 4. Local files win on speed but lose on safety

Local markdown files are unbeatable on raw latency (<1ms). But they have no conflict detection, no audit trail, no freshness guarantee, and no encryption at rest. For a single user editing documents alone, they're perfect. For multiple agents across machines, they're dangerous.
