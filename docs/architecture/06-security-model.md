# Security Model

## Principle of Least Privilege

Each CLI backend is designed with the narrowest possible access scope.

### Google Drive (`gdrive-bench`)

| Layer           | Scope                                              |
|-----------------|-----------------------------------------------------|
| OAuth scope     | `drive.file` — only files created/opened by the app |
| Folder          | `$GDRIVE_FOLDER` — single folder ID                 |
| Token storage   | `~/.config/gdrive-bench/token.json` (mode 0600)     |
| Credentials     | `~/.config/gdrive-bench/credentials.json`            |

### GCS (`gcs-bench`)

| Layer           | Scope                                               |
|-----------------|------------------------------------------------------|
| IAM role        | `roles/storage.objectUser` — read/write objects only |
| Bucket          | Single bucket via IAM binding                        |
| Prefix          | `$GCS_PREFIX` — CLI-level restriction to subfolder   |
| Auth            | Service account key (no browser OAuth)               |
| Key storage     | `~/.config/gcs-bench/sa-key.json` (mode 0600)        |

### Comparison

| Concern                          | Drive                  | GCS                         |
|----------------------------------|------------------------|------------------------------|
| Can access other users' files?   | No (`drive.file`)      | No (bucket-scoped IAM)       |
| Can access other GCP services?   | No (Drive-only scope)  | No (`objectUser` only)       |
| Can delete the bucket?           | N/A                    | No (no `storage.admin`)      |
| Can change IAM permissions?      | No                     | No (no `iam.admin`)          |
| Auth requires browser?           | Yes (OAuth flow)       | No (SA key file)             |
| Token auto-refreshes?            | Yes (refresh token)    | Yes (SA key → JWT)           |

## Data at Rest

### GCS Default Encryption

All objects in GCS are encrypted at rest with AES-256 by default. Google manages the keys automatically.

```
Object data → AES-256 encryption → Stored on disk
              (Google-managed key)
```

### Upgrade Path: Customer-Managed Keys (CMEK)

For organizations that require key control:

```bash
# Create a KMS key
gcloud kms keyrings create bench-keyring --location=me-west1
gcloud kms keys create bench-key \
  --keyring=bench-keyring --location=me-west1 \
  --purpose=encryption

# Set bucket to use CMEK
gcloud storage buckets update gs://my-bucket \
  --default-encryption-key=projects/my-project/locations/me-west1/keyRings/bench-keyring/cryptoKeys/bench-key
```

### Upgrade Path: Client-Side Encryption

For zero-trust (GCP never sees plaintext):

```go
// Encrypt before upload
encrypted := encrypt(content, localKey)
writer.Write(encrypted)

// Decrypt after download
content := decrypt(downloaded, localKey)
```

This is not implemented but the architecture supports it — content is a string going in and out of the cache.

## Data in Transit

- Drive API: HTTPS/TLS 1.3
- GCS API: gRPC with TLS 1.3 (when using Go client library)
- Daemon: Unix socket (local only, no network exposure)

The daemon does NOT listen on TCP. The Unix socket is file-permission protected (mode 0700 on the config directory), accessible only to the user who started it.

## Audit Trail

Every operation is logged with:
- Who (agent ID + machine hostname)
- What (operation + file ID/key)
- When (UTC timestamp with nanosecond precision)
- How (source: api, cache, index)
- Result (ok, error, conflict)
- Latency

Log file: `~/.config/{gdrive,gcs}-bench/audit.jsonl` (mode 0600, append-only)

## Credential Rotation

### Drive
```bash
# Re-run OAuth flow — old token is revoked
./gdrive-bench auth
```

### GCS
```bash
# Delete old key, create new one
gcloud iam service-accounts keys delete OLD_KEY_ID \
  --iam-account=gcs-bench@project.iam.gserviceaccount.com
gcloud iam service-accounts keys create ~/.config/gcs-bench/sa-key.json \
  --iam-account=gcs-bench@project.iam.gserviceaccount.com
# Restart daemon to pick up new key
./gcs-bench daemon stop && ./gcs-bench daemon start
```
