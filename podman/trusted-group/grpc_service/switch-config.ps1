# Switches the already-deployed ubertool-service container to a different config scenario
# without rebuilding the image — just recreates the container with a different -config arg.
#
# Usage:
#   .\switch-config.ps1 -Mode precommit  # A1: no TLS, no FCM, 2FA bypassed
#   .\switch-config.ps1 -Mode uitest     # A2: FCM on, 2FA bypassed (desktop UI automated tests)
#   .\switch-config.ps1 -Mode manual     # A3: FCM on, live 2FA email (desktop manual/QA testing)
#
# Requires `make ubertool-start` to have been run at least once (the ubertool-service image
# must already exist).
param(
    [Parameter(Mandatory=$true)]
    [ValidateSet("precommit", "uitest", "manual")]
    [string]$Mode
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

switch ($Mode) {
    "precommit" { $ConfigFile = "config.precommit.yaml" }
    "uitest"    { $ConfigFile = "config.desktop.uitest.yaml" }
    "manual"    { $ConfigFile = "config.desktop.manual.yaml" }
}

podman image exists $IMAGE_NAME
if ($LASTEXITCODE -ne 0) {
    Write-Error "Image '$IMAGE_NAME' not found. Run 'make ubertool-start' first."
    exit 1
}

Write-Host "Switching ubertool-service to '$Mode' (config/$ConfigFile)..."

$containerExists = podman ps -a --format "{{.Names}}" | Select-String -Pattern "^$CONTAINER_NAME$"
if ($containerExists) {
    podman stop $CONTAINER_NAME | Out-Null
    podman rm $CONTAINER_NAME | Out-Null
}

podman run -d `
  --name $CONTAINER_NAME `
  --add-host "host.containers.internal:host-gateway" `
  -p "${GRPC_PORT}:50052" `
  -p "${STORAGE_PORT}:50053" `
  -e "DB_HOST=$DB_HOST" `
  -e "DB_PORT=$DB_PORT" `
  -v "${RepoRoot}\config:/app/config:ro" `
  --restart unless-stopped `
  $IMAGE_NAME "-config=/app/config/$ConfigFile"

if ($LASTEXITCODE -ne 0) {
    Write-Error "podman run failed."
    exit 1
}

Write-Host ""
Write-Host "Done. ubertool-service is now running with '$ConfigFile' ($Mode)."
Write-Host "View logs: podman logs -f $CONTAINER_NAME"
