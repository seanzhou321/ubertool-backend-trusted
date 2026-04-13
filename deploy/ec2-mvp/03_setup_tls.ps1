# =============================================================================
# 03_setup_tls.ps1
# Installs Certbot on EC2 and obtains a Let's Encrypt TLS certificate.
# Run this only after registering a domain and pointing its A-record
# to the Elastic IP.
#
# Usage: .\03_setup_tls.ps1

$ErrorActionPreference = "Stop"

. "$PSScriptRoot\Load-Env.ps1"
Import-Env "$PSScriptRoot\config.env"
Import-Env "$PSScriptRoot\infra_state.env"

Write-Host "========================================"
Write-Host " Ubertool MVP - TLS Certificate Setup"
Write-Host "========================================"
Write-Host "  Domain : $DOMAIN"
Write-Host "  Email  : $CERTBOT_EMAIL"
Write-Host ""

# 1. Verify DNS resolution
Write-Host "[1/3] Checking DNS resolution for $DOMAIN..."
try {
    $resolved = (Resolve-DnsName -Name $DOMAIN -Type A -ErrorAction Stop).IPAddress
} catch {
    $resolved = ""
}

if ($resolved -ne $ELASTIC_IP) {
    Write-Host ""
    Write-Host "  WARNING: $DOMAIN resolves to '$resolved'"
    Write-Host "           Expected Elastic IP: $ELASTIC_IP"
    Write-Host ""
    $confirm = Read-Host "  Continue anyway? (y/N)"
    if ($confirm -notmatch '^[Yy]$') {
        Write-Host "Aborted."
        exit 1
    }
}

# 2. Install Certbot and obtain certificate on EC2
Write-Host "[2/3] Installing Certbot and obtaining certificate on EC2..."
$remoteScript = "set -euo pipefail
sudo apt-get update -qq
sudo apt-get install -y -qq certbot
sudo fuser -k 80/tcp 2>/dev/null || true
sudo certbot certonly --standalone --non-interactive --agree-tos --email '$CERTBOT_EMAIL' -d '$DOMAIN' --cert-name ubertool
sudo systemctl enable certbot.timer
sudo systemctl start certbot.timer
sudo mkdir -p /etc/ubertool/certs
sudo cp /etc/letsencrypt/live/ubertool/fullchain.pem /etc/ubertool/certs/
sudo cp /etc/letsencrypt/live/ubertool/privkey.pem   /etc/ubertool/certs/
sudo chmod 644 /etc/ubertool/certs/fullchain.pem
sudo chmod 640 /etc/ubertool/certs/privkey.pem
sudo chown root:ubuntu /etc/ubertool/certs/privkey.pem
echo 'TLS setup complete.'"

ssh -i $SSH_KEY_PATH -o StrictHostKeyChecking=no "ubuntu@$ELASTIC_IP" $remoteScript

# 3. Summary
Write-Host "[3/3] Certificate provisioned."
Write-Host ""
Write-Host "========================================"
Write-Host " TLS Setup Complete!"
Write-Host "========================================"
Write-Host ""
Write-Host "  Certificate : /etc/ubertool/certs/fullchain.pem"
Write-Host "  Private key : /etc/ubertool/certs/privkey.pem"
Write-Host "  Auto-renews : systemd certbot.timer"
Write-Host ""
Write-Host "Next step: Run .\04_deploy.ps1"
