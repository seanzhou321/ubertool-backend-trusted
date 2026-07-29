# Podman deployment script for the Ubertool Trusted Backend gRPC server.
# Builds the image from the repo root and runs it against the already-running
# ubertool-postgres container (see podman/trusted-group/postgres/install.ps1),
# reached via Podman's host-gateway alias since the two containers don't share
# a user-defined network.
#
# To switch config scenario on an already-built image without rebuilding, use
# switch-config.ps1 (make ubertool-use-precommit / ubertool-use-uitest / ubertool-use-manual)
# instead of re-running this script.
param(
    [string]$ConfigFile = "config.precommit.yaml"
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot  = Resolve-Path (Join-Path $ScriptDir "..\..\..")

$IMAGE_NAME     = "ubertool-service"
$CONTAINER_NAME = "ubertool-service"
$GRPC_PORT      = 50052
$STORAGE_PORT   = 50053
$DB_HOST        = "host.containers.internal"
$DB_PORT        = 5454

# Generated proto code is gitignored — the Dockerfile's `COPY . .` needs it present locally.
if (-not (Test-Path (Join-Path $RepoRoot "api\gen\v1"))) {
    Write-Error "api/gen/v1 not found. Run 'make proto-gen' (requires protoc) before deploying."
    exit 1
}

Write-Host "Building Podman image '$IMAGE_NAME'..."
podman build -t $IMAGE_NAME -f (Join-Path $ScriptDir "Dockerfile") $RepoRoot
if ($LASTEXITCODE -ne 0) {
    Write-Error "podman build failed."
    exit 1
}

# Stop and remove any existing container so re-running this script is idempotent.
$containerExists = podman ps -a --format "{{.Names}}" | Select-String -Pattern "^$CONTAINER_NAME$"
if ($containerExists) {
    Write-Host "Stopping and removing existing container..."
    podman stop $CONTAINER_NAME | Out-Null
    podman rm $CONTAINER_NAME | Out-Null
}

Write-Host "Starting container '$CONTAINER_NAME'..."
# Config is mounted at /app/config (not /config) so configs' relative paths — e.g.
# firebase_key_path: "config/firebase-admin-key.json" in the desktop uitest/manual configs —
# resolve correctly against the container's /app working directory.
#
# uploads/ is bind-mounted too (writable): storage.upload_dir in every config is a relative
# path ("./uploads") that resolves to /app/uploads inside the container. Without this mount
# that directory only exists in the container's writable layer — invisible to anything
# running on the host (e.g. tests/e2e's image_storage_test.go, which reads/writes the host
# repo's uploads/ directly) and wiped every time the container is recreated.
if (-not (Test-Path (Join-Path $RepoRoot "uploads"))) {
    New-Item -ItemType Directory -Path (Join-Path $RepoRoot "uploads") | Out-Null
}
podman run -d `
  --name $CONTAINER_NAME `
  --add-host "host.containers.internal:host-gateway" `
  -p "${GRPC_PORT}:50052" `
  -p "${STORAGE_PORT}:50053" `
  -e "DB_HOST=$DB_HOST" `
  -e "DB_PORT=$DB_PORT" `
  -v "${RepoRoot}\config:/app/config:ro" `
  -v "${RepoRoot}\uploads:/app/uploads" `
  --restart unless-stopped `
  $IMAGE_NAME "-config=/app/config/$ConfigFile"

if ($LASTEXITCODE -ne 0) {
    Write-Error "podman run failed."
    exit 1
}

Write-Host ""
Write-Host "Ubertool backend server deployed successfully!"
Write-Host "gRPC:         localhost:$GRPC_PORT"
Write-Host "Mock storage: http://localhost:$STORAGE_PORT"
Write-Host "Config:       config/$ConfigFile (mounted read-only; DB_HOST/DB_PORT overridden to reach"
Write-Host "              the host-mapped ubertool-postgres container at ${DB_HOST}:${DB_PORT})"
Write-Host ""
Write-Host "View logs:    podman logs -f $CONTAINER_NAME"
Write-Host "Stop:         make ubertool-stop"
Write-Host "Switch config: make ubertool-use-precommit / ubertool-use-uitest / ubertool-use-manual"
