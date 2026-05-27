# Per-User Permissions

## Model

Each user or team gets an isolated prefix (folder) in the GCS bucket. Permissions are enforced at two layers:

1. **GCS IAM** (hard boundary) — a per-user service account with IAM Conditions restricting access to their prefix. Can't be bypassed even with `gsutil` or the GCS console.
2. **CLI** (UX layer) — the daemon validates the configured prefix before executing. Returns friendly errors instead of cryptic IAM denials.

```
Bucket: org-documents
├── team-alpha/           ← Alice's SA can only access this
│   ├── q1-revenue.md
│   └── roadmap.md
├── team-beta/            ← Bob's SA can only access this
│   ├── compliance.md
│   └── audit-report.md
├── team-gamma/           ← Charlie's SA can only access this
│   └── hiring-plan.md
└── shared/               ← All SAs can read this (not write)
    ├── company-policy.md
    └── templates/
```

## IAM Setup (hard boundary)

### Step 1: Create per-user service accounts

```bash
PROJECT=my-gcp-project
BUCKET=org-documents

# One SA per user/team
gcloud iam service-accounts create gcs-bench-alice \
  --display-name="gcs-bench: Alice (team-alpha)"

gcloud iam service-accounts create gcs-bench-bob \
  --display-name="gcs-bench: Bob (team-beta)"
```

### Step 2: Grant prefix-scoped access with IAM Conditions

GCS supports IAM Conditions on `resource.name` to restrict access to a specific prefix:

```bash
# Alice: read+write on team-alpha/ only
gcloud storage buckets add-iam-policy-binding gs://$BUCKET \
  --member="serviceAccount:gcs-bench-alice@$PROJECT.iam.gserviceaccount.com" \
  --role="roles/storage.objectUser" \
  --condition="expression=resource.name.startsWith('projects/_/buckets/$BUCKET/objects/team-alpha/'),title=team-alpha-only"

# Alice: read-only on shared/
gcloud storage buckets add-iam-policy-binding gs://$BUCKET \
  --member="serviceAccount:gcs-bench-alice@$PROJECT.iam.gserviceaccount.com" \
  --role="roles/storage.objectViewer" \
  --condition="expression=resource.name.startsWith('projects/_/buckets/$BUCKET/objects/shared/'),title=shared-read-only"

# Bob: read+write on team-beta/ only
gcloud storage buckets add-iam-policy-binding gs://$BUCKET \
  --member="serviceAccount:gcs-bench-bob@$PROJECT.iam.gserviceaccount.com" \
  --role="roles/storage.objectUser" \
  --condition="expression=resource.name.startsWith('projects/_/buckets/$BUCKET/objects/team-beta/'),title=team-beta-only"

# Bob: read-only on shared/
gcloud storage buckets add-iam-policy-binding gs://$BUCKET \
  --member="serviceAccount:gcs-bench-bob@$PROJECT.iam.gserviceaccount.com" \
  --role="roles/storage.objectViewer" \
  --condition="expression=resource.name.startsWith('projects/_/buckets/$BUCKET/objects/shared/'),title=shared-read-only"
```

### Step 3: Generate per-user SA keys

```bash
gcloud iam service-accounts keys create ~/.config/gcs-bench/sa-key.json \
  --iam-account=gcs-bench-alice@$PROJECT.iam.gserviceaccount.com
```

Each user's machine has only their own SA key. Even if they know another team's prefix, IAM blocks the request.

### What Each User Can Do

| Action | Own prefix (team-alpha/) | Other prefix (team-beta/) | Shared (shared/) |
|--------|-------------------------|--------------------------|-------------------|
| List objects | Yes | **No** (403) | Yes |
| Read content | Yes | **No** (403) | Yes |
| Create object | Yes | **No** (403) | **No** (403) |
| Update object | Yes | **No** (403) | **No** (403) |
| Delete object | Yes | **No** (403) | **No** (403) |
| See bucket exists | Yes | Yes (bucket-level) | Yes |

## CLI Enforcement (UX layer)

The CLI validates operations against the user's configured prefix before sending to GCS. This catches mistakes early with clear messages instead of opaque IAM errors.

### Config file

```json
{
  "bucket": "org-documents",
  "user_prefix": "team-alpha",
  "shared_prefixes": ["shared"],
  "agent_id": "alice-clinic-a",
  "credentials": "~/.config/gcs-bench/sa-key.json"
}
```

### CLI validation logic

```go
func validateAccess(key string, operation string) error {
    cfg := loadConfig()

    userPrefix := cfg.UserPrefix + "/"
    if strings.HasPrefix(key, userPrefix) {
        return nil // user's own prefix — all operations allowed
    }

    for _, shared := range cfg.SharedPrefixes {
        if strings.HasPrefix(key, shared+"/") {
            if operation == "read" || operation == "find" || operation == "search" {
                return nil // shared prefix — read operations allowed
            }
            return fmt.Errorf(
                "permission denied: %s is in shared/ (read-only). "+
                "You can read but not write to shared documents.", key)
        }
    }

    return fmt.Errorf(
        "permission denied: %s is outside your prefix (%s). "+
        "You can only access files in %s/ and shared/.",
        key, cfg.UserPrefix, cfg.UserPrefix)
}
```

### What the agent sees

```bash
# Alice tries to read Bob's file
$ gcs-bench read --key "team-beta/compliance.md"
{"error": "permission denied: team-beta/compliance.md is outside your prefix (team-alpha). You can only access files in team-alpha/ and shared/."}

# Alice reads from shared (OK)
$ gcs-bench read --key "shared/company-policy.md"
{"name": "company-policy.md", "content": "...", "_elapsed_ms": 42}

# Alice tries to write to shared (denied)
$ gcs-bench create --name "shared/my-file.md" --body "test"
{"error": "permission denied: shared/my-file.md is in shared/ (read-only). You can read but not write to shared documents."}

# Even if Alice bypasses the CLI and uses gsutil:
$ gsutil cat gs://org-documents/team-beta/compliance.md
AccessDeniedException: 403 gcs-bench-alice@... does not have storage.objects.get access
```

## Onboarding a New User

```bash
# Admin runs this once per user
gcs-bench admin add-user \
  --user charlie \
  --prefix team-gamma \
  --project my-gcp-project \
  --bucket org-documents
```

This would:
1. Create SA `gcs-bench-charlie@project.iam.gserviceaccount.com`
2. Grant `objectUser` on `team-gamma/` + `objectViewer` on `shared/`
3. Generate SA key
4. Write config file with `user_prefix: team-gamma`
5. Run smoke test (list objects in prefix)

## Audit Trail

Permissions are logged alongside every operation:

```json
{
  "timestamp": "2026-05-27T06:30:00Z",
  "agent_id": "alice-clinic-a",
  "machine": "alices-macbook.local",
  "operation": "read",
  "key": "team-alpha/q1-revenue.md",
  "result": "ok",
  "user_prefix": "team-alpha",
  "latency_ms": 45
}
```

```json
{
  "timestamp": "2026-05-27T06:30:05Z",
  "agent_id": "alice-clinic-a",
  "machine": "alices-macbook.local",
  "operation": "read",
  "key": "team-beta/compliance.md",
  "result": "denied",
  "detail": "outside user prefix (team-alpha)",
  "user_prefix": "team-alpha",
  "latency_ms": 0
}
```

## Security Properties

| Property | Guarantee |
|----------|-----------|
| User A can't read User B's files | IAM-enforced (server-side, can't bypass) |
| User A can't write to shared/ | IAM-enforced |
| Friendly error messages | CLI-enforced (pre-flight check) |
| All access attempts are logged | Audit log (including denied attempts) |
| SA key compromise scope | Limited to one user's prefix only |
| Revoking a user | Delete SA → instant, all access stops |

## Comparison with Alternatives

| Aspect | This design | Mirage | gcsfuse |
|--------|------------|--------|---------|
| Isolation boundary | IAM + CLI prefix | Mount modes (READ/WRITE per path) | Mount-level (entire bucket) |
| Enforcement | Server-side (GCS IAM) | Client-side (workspace config) | Client-side (mount options) |
| Can be bypassed? | No (IAM) | Yes (direct API access) | Yes (gsutil) |
| Shared read-only area | IAM Conditions | MountMode.READ | Separate mount |
| Per-user audit | Yes (agent ID in log) | No | No |
| Admin tooling | `gcs-bench admin add-user` | Manual workspace config | Manual mount config |
