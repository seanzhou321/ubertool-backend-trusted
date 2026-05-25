# =============================================================================
# 05_setup_data.ps1  (EC2-only — runs from your local Windows machine)
# Cross-compiles the data setup binary for Linux, uploads it to EC2, and
# seeds the RDS database using the supplied data YAML.
#
# The app config (/etc/ubertool/config.yaml) always lives on EC2; only the
# seed data file is environment-agnostic and comes from this repo.
#
# Usage:
#   .\05_setup_data.ps1                                          # default data file
#   .\05_setup_data.ps1 -DataFile tests\data-setup\user_org.test.yaml
# =============================================================================

param(
    # Path to the seed data YAML, relative to project root or absolute.
    [string]$DataFile = "tests\data-setup\user_org.ec2-mvp.yaml"
)

$ErrorActionPreference = "Stop"

. "$PSScriptRoot\Load-Env.ps1"
Import-Env "$PSScriptRoot\config.env"
Import-Env "$PSScriptRoot\infra_state.env"

$PROJECT_ROOT = Resolve-Path "$PSScriptRoot\..\.."
$BUILD_DIR    = "$PSScriptRoot\build"

# Resolve the local data file path (relative paths are relative to project root)
if (-not [System.IO.Path]::IsPathRooted($DataFile)) {
    $DataFile = Join-Path $PROJECT_ROOT $DataFile
}
$LocalDataFile  = Resolve-Path $DataFile
$RemoteDataFile = "/tmp/$([System.IO.Path]::GetFileName($LocalDataFile))"

Write-Host "========================================"
Write-Host " Ubertool MVP - Populate Initial Data"
Write-Host "========================================"
Write-Host "  Target    : ubuntu@$ELASTIC_IP"
Write-Host "  Data file : $LocalDataFile"
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

# 2. Upload binary and data YAML to EC2
Write-Host "[2/3] Uploading to EC2..."
$scpArgs = "-i `"$SSH_KEY_PATH`" -o StrictHostKeyChecking=no `"$BUILD_DIR\setup-data`" `"$LocalDataFile`" ubuntu@${ELASTIC_IP}:/tmp/"
cmd /c "C:\Windows\System32\OpenSSH\scp.exe $scpArgs"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# 3. Run on EC2 — app config lives on EC2; data file was just uploaded to /tmp/
Write-Host "[3/3] Running setup on EC2..."
$sshArgs = "-i `"$SSH_KEY_PATH`" -o StrictHostKeyChecking=no ubuntu@$ELASTIC_IP `"chmod +x /tmp/setup-data && sudo /tmp/setup-data -config=/etc/ubertool/config.yaml -setup=$RemoteDataFile`""
cmd /c "C:\Windows\System32\OpenSSH\ssh.exe $sshArgs"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ""
Write-Host "========================================"
Write-Host " Initial data populated successfully!"
Write-Host "========================================"
