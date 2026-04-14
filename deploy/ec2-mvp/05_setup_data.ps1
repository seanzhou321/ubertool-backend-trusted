# =============================================================================
# 05_setup_data.ps1
# Cross-compiles the data setup binary for Linux, uploads it to EC2 together
# with the ec2-mvp YAML, and runs it against the RDS database.
#
# Usage:
#   .\05_setup_data.ps1
# =============================================================================

$ErrorActionPreference = "Stop"

. "$PSScriptRoot\Load-Env.ps1"
Import-Env "$PSScriptRoot\config.env"
Import-Env "$PSScriptRoot\infra_state.env"

$PROJECT_ROOT = Resolve-Path "$PSScriptRoot\..\.."
$BUILD_DIR    = "$PSScriptRoot\build"

Write-Host "========================================"
Write-Host " Ubertool MVP - Populate Initial Data"
Write-Host "========================================"
Write-Host "  Target : ubuntu@$ELASTIC_IP"
Write-Host ""

# 1. Cross-compile setup binary for Linux
Write-Host "[1/3] Building setup binary (linux/amd64)..."
New-Item -ItemType Directory -Force -Path $BUILD_DIR | Out-Null

Push-Location $PROJECT_ROOT
$env:GOOS        = "linux"
$env:GOARCH      = "amd64"
$env:CGO_ENABLED = "0"
go build -o "$BUILD_DIR\setup-data" ./tests/data-setup/setup.go
Remove-Item Env:\GOOS
Remove-Item Env:\GOARCH
Remove-Item Env:\CGO_ENABLED
Pop-Location
Write-Host "      Binary: $BUILD_DIR\setup-data"

# 2. Upload binary and YAML to EC2
Write-Host "[2/3] Uploading to EC2..."
$scpArgs = "-i `"$SSH_KEY_PATH`" -o StrictHostKeyChecking=no `"$BUILD_DIR\setup-data`" `"$PROJECT_ROOT\tests\data-setup\user_org.ec2-mvp.yaml`" ubuntu@${ELASTIC_IP}:/tmp/"
cmd /c "C:\Windows\System32\OpenSSH\scp.exe $scpArgs"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# 3. Run on EC2
Write-Host "[3/3] Running setup on EC2..."
$sshArgs = "-i `"$SSH_KEY_PATH`" -o StrictHostKeyChecking=no ubuntu@$ELASTIC_IP `"chmod +x /tmp/setup-data && sudo /tmp/setup-data -setup=/tmp/user_org.ec2-mvp.yaml`""
cmd /c "C:\Windows\System32\OpenSSH\ssh.exe $sshArgs"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ""
Write-Host "========================================"
Write-Host " Initial data populated successfully!"
Write-Host "========================================"
