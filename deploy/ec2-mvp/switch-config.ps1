# =============================================================================
# switch-config.ps1
# Uploads one of the repo config YAMLs to EC2 as /etc/ubertool/config.yaml
# and restarts the ubertool-api service.
#
# Usage:
#   .\switch-config.ps1 -Mode prod     # production (2FA on, real email, S3)
#   .\switch-config.ps1 -Mode uitest   # UI automated tests (2FA bypassed, mock email)
# =============================================================================

param(
    [Parameter(Mandatory=$true)]
    [ValidateSet("prod", "uitest")]
    [string]$Mode
)

$ErrorActionPreference = "Stop"

. "$PSScriptRoot\Load-Env.ps1"
Import-Env "$PSScriptRoot\config.env"
Import-Env "$PSScriptRoot\infra_state.env"

$PROJECT_ROOT = Resolve-Path "$PSScriptRoot\..\.."

switch ($Mode) {
    "prod"   { $ConfigFile = "$PROJECT_ROOT\config\config.ec2.prod.yaml" }
    "uitest" { $ConfigFile = "$PROJECT_ROOT\config\config.ec2.uitest.yaml" }
}

Write-Host "========================================"
Write-Host " Ubertool MVP - Switch EC2 Config"
Write-Host "========================================"
Write-Host "  Mode        : $Mode"
Write-Host "  Config file : $ConfigFile"
Write-Host "  Target      : ubuntu@$ELASTIC_IP"
Write-Host ""

# Upload config (rename to config.yaml at destination)
Write-Host "[1/2] Uploading config to EC2..."
$scpArgs = @("-i", $SSH_KEY_PATH, "-o", "StrictHostKeyChecking=no", $ConfigFile, "ubuntu@${ELASTIC_IP}:/tmp/config.yaml")
& scp @scpArgs
if ($LASTEXITCODE -ne 0) { throw "scp failed with exit code $LASTEXITCODE" }

# Install and restart
Write-Host "[2/2] Installing config and restarting service..."
$remoteCmd = @"
set -euo pipefail
sudo mv /tmp/config.yaml /etc/ubertool/config.yaml
sudo chown root:ubertool /etc/ubertool/config.yaml
sudo chmod 640 /etc/ubertool/config.yaml
sudo systemctl restart ubertool-api
sleep 2
sudo systemctl status ubertool-api --no-pager
"@
ssh -i $SSH_KEY_PATH -o StrictHostKeyChecking=no "ubuntu@$ELASTIC_IP" $remoteCmd
if ($LASTEXITCODE -ne 0) { throw "Remote command failed with exit code $LASTEXITCODE" }

Write-Host ""
Write-Host "Done. EC2 is now running with the '$Mode' config."
