# Architecture Documentation

1. [Overview](01-overview.md) — system diagram, project structure, performance summary
2. [Daemon Architecture](02-daemon-architecture.md) — why a daemon, lifecycle, auto-routing, warmup timeline
3. [Caching and Freshness](03-caching-and-freshness.md) — cache layers, ETag validation, freshness guarantees
4. [Concurrency and Safety](04-concurrency-and-safety.md) — optimistic concurrency, CAS, audit logging, failure modes
5. [Multi-User Shipping](05-multi-user-shipping.md) — what to build to ship to a team, MVP/production/enterprise checklists
6. [Security Model](06-security-model.md) — least privilege, encryption at rest/transit, credential management
7. [Benchmark Results](07-benchmark-results.md) — measured latency, optimization journey, key insights
8. [Alternatives](08-alternatives.md) — off-the-shelf options (Mirage, Nexus, gcsfuse, rclone, MCP servers) and when to use them vs. this project
9. [Deep Dive: Mirage and gcsfuse](09-deep-dive-mirage-gcsfuse.md) — setup, architecture, caching, limitations, and head-to-head comparison of the two most practical alternatives
10. [Per-User Permissions](10-per-user-permissions.md) — folder-level isolation with GCS IAM Conditions + CLI validation, onboarding, audit trail
