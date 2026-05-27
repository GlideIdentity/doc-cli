# Daemon Architecture

## Why a Daemon

Each CLI invocation without a daemon pays these costs:

| Cost                  | Per-call overhead |
|-----------------------|-------------------|
| Process startup       | ~5-10ms           |
| OAuth token load      | ~1-5ms            |
| TLS handshake         | ~50-100ms         |
| DNS resolution        | ~10-30ms          |
| API request           | ~300-500ms        |

The daemon eliminates the first four by keeping a persistent process with a pre-warmed connection pool.

## How It Works

```
┌──────────────────────────────────────┐
│              Daemon Process          │
│                                      │
│  ┌──────────┐  ┌──────────────────┐  │
│  │ HTTP Mux │  │ Pre-warmed       │  │
│  │          │  │ API Client       │  │
│  │ /find    │  │ (TLS session,    │  │
│  │ /read    │  │  OAuth token,    │  │
│  │ /search  │  │  HTTP/2 conn)    │  │
│  │ /create  │  │                  │  │
│  │ /update  │  └──────────────────┘  │
│  │ /health  │                        │
│  └────┬─────┘  ┌──────────────────┐  │
│       │        │ Content Cache    │  │
│       │        │ (in-memory,      │  │
│       │        │  ETag validated) │  │
│  ┌────┴─────┐  └──────────────────┘  │
│  │ Unix     │                        │
│  │ Socket   │  ┌──────────────────┐  │
│  │ Listener │  │ Local Index      │  │
│  └────┬─────┘  │ (file list +    │  │
│       │        │  content in RAM) │  │
│       │        └──────────────────┘  │
│       │                              │
│       │        ┌──────────────────┐  │
│       │        │ Audit Logger     │  │
│       │        │ (append-only     │  │
│       │        │  JSONL file)     │  │
│       │        └──────────────────┘  │
└───────┼──────────────────────────────┘
        │
   Unix Socket
   ~/.config/{gdrive,gcs}-bench/daemon.sock
        │
┌───────┴──────┐
│ CLI Binary   │
│ (short-lived │
│  process)    │
│              │
│ Detects sock │
│ → routes via │
│   daemon     │
└──────────────┘
```

## Lifecycle

```
1. `*-bench daemon start`
   → Creates Unix socket
   → Initializes API client (TLS handshake, token load)
   → Warms connection pool (lightweight API call)
   → Starts background index build:
     a. Lists all files/objects in the folder/bucket
     b. Spawns goroutines to prefetch content for each file
     c. Once content is ready, populates the content cache
   → Writes PID file
   → Starts HTTP server on Unix socket

2. `*-bench find --name "revenue"`
   → CLI checks if daemon.sock exists
   → If yes: routes request to daemon via HTTP over Unix socket
   → If no: creates a fresh API client and calls API directly

3. `*-bench daemon stop`
   → Reads PID file
   → Sends SIGTERM
   → Daemon closes audit log, cleans up socket and PID file
```

## Auto-Routing

The CLI binary transparently routes through the daemon when it's running:

```go
case "find":
    if isDaemonRunning() {
        err = runFindViaDaemon(os.Args[2:])  // Unix socket → daemon
    } else {
        err = runFind(os.Args[2:])           // Direct API call
    }
```

The agent doesn't need to know whether the daemon is running. Same command, same output — just faster when the daemon is warm.

## Startup Warmup Timeline

```
T+0ms     Daemon process starts
T+100ms   API client initialized, TLS established
T+500ms   Connection pool warmed (About.Get / Bucket.Attrs)
T+800ms   File list fetched (index metadata ready)
T+800ms   Find/Search now served from local index (0ms)
T+3-8s    All file content prefetched into cache
T+8s      Read now served from cache with ETag validation (~50-500ms)
          Full system warm — all operations at peak speed
```
