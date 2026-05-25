# =============================================================================
# 06_wipe_data.ps1  (EC2-only — runs from your local Windows machine)
# Cross-compiles the data setup binary for Linux, uploads it to EC2, and
# truncates all tables while leaving the schema intact.
#
# No data YAML is needed — only the app config on EC2 is used to connect.
# Run 05_setup_data.ps1 afterwards to restore the baseline org and admin user.
#
# Usage:
#   .\06_wipe_data.ps1
# =============================================================================

$ErrorActionPreference = "Stop"

. "$PSScriptRoot\Load-Env.ps1"
Import-Env "$PSScriptRoot\config.env"
Import-Env "$PSScriptRoot\infra_state.env"

$PROJECT_ROOT = Resolve-Path "$PSScriptRoot\..\.."
$BUILD_DIR    = "$PSScriptRoot\build"

Write-Host "========================================"
Write-Host " Ubertool MVP - Wipe All Data"
Write-Host "========================================"
Write-Host "  Target  : ubuntu@$ELASTIC_IP"
Write-Host "  WARNING : All rows will be deleted. Schema is preserved."
Write-Host ""

# 1. Cross-compile setup binary for Linux
# Environment variables are set only for the duration of the build, then restored.
Write-Host "[1/3] Building setup binary (linux/amd64)..."
New-Item -ItemType Directory -Force -Path $BUILD_DIR | Out-Null
Push-Location $PROJECT_ROOT
try {
    $env:GOOS        = "linux"
    $env:GOARCH      = "amd64"
    $env:CGO_ENABLED = "0"
    go build -o "$BUILD_DIR\setup-data" ./tests/data-setup/setup.go
} finally {
    Remove-Item Env:\GOOS, Env:\GOARCH, Env:\CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}
Write-Host "      Binary: $BUILD_DIR\setup-data"

# 2. Upload binary to EC2 (no data YAML needed — wipe only needs the DB config on EC2)
Write-Host "[2/3] Uploading to EC2..."
$scpArgs = "-i `"$SSH_KEY_PATH`" -o StrictHostKeyChecking=no `"$BUILD_DIR\setup-data`" ubuntu@${ELASTIC_IP}:/tmp/"
cmd /c "C:\Windows\System32\OpenSSH\scp.exe $scpArgs"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# 3. Run on EC2 — app config lives on EC2; no data file required for wipe
Write-Host "[3/3] Wiping data on EC2..."
$sshArgs = "-i `"$SSH_KEY_PATH`" -o StrictHostKeyChecking=no ubuntu@$ELASTIC_IP `"chmod +x /tmp/setup-data && sudo /tmp/setup-data -config=/etc/ubertool/config.yaml -wipe`""
cmd /c "C:\Windows\System32\OpenSSH\ssh.exe $sshArgs"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ""
Write-Host "========================================"
Write-Host " All data wiped successfully!"
Write-Host " Run .\05_setup_data.ps1 to restore baseline data."
Write-Host "========================================"
