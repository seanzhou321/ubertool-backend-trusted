# Ubertool Backend Service Container

Local Podman deployment of the Ubertool Trusted Backend gRPC server (`cmd/server`) — the
same binary `make run-precommit` runs directly on the desktop, just containerized.

## Prerequisites

- The `ubertool-postgres` container must already be running (`make db-deploy`).
- Generated proto code must exist at `api/gen/v1` (`make proto-gen`) — it's gitignored, so
  the Dockerfile's `COPY . .` needs it present on disk before building.

## Deploy / Teardown

```bash
make ubertool-start   # build image + run container
make ubertool-stop    # stop + remove container and image
```

Equivalent direct invocation:

```powershell
powershell -File podman\trusted-group\grpc_service\install.ps1
powershell -File podman\trusted-group\grpc_service\teardown.ps1
```

## What it does

- Builds a two-stage image (`golang:1.25-alpine` builder → `alpine:latest` runtime) from
  the repo root, producing a single `/app/server` binary.
- Runs it as container `ubertool-service`, publishing:
  - `50052` — gRPC (matches `config/config.precommit.yaml`'s `server.port`)
  - `50053` — mock-storage HTTP file server (matches `storage.base_url`)
- Mounts `config/` read-only at `/app/config` and runs with
  `-config=/app/config/config.precommit.yaml` (no TLS, no FCM, 2FA bypassed, mock SMTP/storage —
  see `config/README.md` for the full scenario matrix). The mount point is `/app/config`, not
  `/config`, so relative paths inside the config files — e.g. `firebase_key_path:
  "config/firebase-admin-key.json"` in the uitest/manual configs — resolve correctly against
  the container's `/app` working directory.
- The server and `ubertool-postgres` are **not** on a shared Podman network (the Postgres
  container uses Podman's default `pasta` networking, not a joinable bridge), so the
  container reaches Postgres via `host.containers.internal:5454` — the `DB_HOST`/`DB_PORT`
  environment variables override the mounted config's `localhost:5454` for this purpose only
  (see `internal/config/config.go`'s env-var overrides).
- Re-running `make ubertool-start` is idempotent: it rebuilds the image and replaces the
  existing container.

## Logs / status

```bash
podman logs -f ubertool-service
podman ps --filter name=ubertool-service
```

## Switching config scenario

`install.ps1` always deploys `config.precommit.yaml`. To switch the **already-running**
container to a different scenario without rebuilding the image, use `switch-config.ps1`
(recreates the container from the existing image with a different `-config` arg):

```bash
make ubertool-use-precommit   # A1: no TLS, no FCM, 2FA bypassed
make ubertool-use-uitest      # A2: FCM on, 2FA bypassed (desktop UI automated tests)
make ubertool-use-manual      # A3: FCM on, live 2FA email (desktop manual/QA testing)
```

`uitest`/`manual` need `config/firebase-admin-key.json` to exist for FCM, and `manual` sends
real 2FA email via the Gmail credentials already baked into `config.desktop.manual.yaml` —
neither is wired up or validated by this script beyond mounting the file that's there.
