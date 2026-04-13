# =============================================================================
# 04_deploy.ps1
# Cross-compiles the Go gRPC binary for Linux/amd64, uploads it to EC2,
# writes the environment config, installs the systemd service, and starts it.
#
# Run this for every code update (redeployment).
#
# Usage:
#   .\04_deploy.ps1
# =============================================================================

$ErrorActionPreference = "Stop"

. "$PSScriptRoot\Load-Env.ps1"
Import-Env "$PSScriptRoot\config.env"
Import-Env "$PSScriptRoot\infra_state.env"

$PROJECT_ROOT = Resolve-Path "$PSScriptRoot\..\.."
$BINARY_NAME  = "ubertool-api"
$BUILD_DIR    = "$PSScriptRoot\build"

Write-Host "========================================"
Write-Host " Ubertool MVP - Build and Deploy"
Write-Host "========================================"
Write-Host "  Target : ubuntu@$ELASTIC_IP"
Write-Host "  Binary : $BINARY_NAME"
Write-Host ""

# 1. Cross-compile Go binary for Linux amd64
Write-Host "[1/6] Building Go binary (linux/amd64)..."
New-Item -ItemType Directory -Force -Path $BUILD_DIR | Out-Null

Push-Location $PROJECT_ROOT
$env:GOOS        = "linux"
$env:GOARCH      = "amd64"
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w" -o "$BUILD_DIR\$BINARY_NAME" ./cmd/server
Remove-Item Env:\GOOS
Remove-Item Env:\GOARCH
Remove-Item Env:\CGO_ENABLED
Pop-Location

$binarySize = [math]::Round((Get-Item "$BUILD_DIR\$BINARY_NAME").Length / 1MB, 1)
Write-Host "      Binary: $BUILD_DIR\$BINARY_NAME ($binarySize MB)"

# 2. Write environment file
Write-Host "[2/6] Writing app environment file..."
$envContent = @(
    "# Ubertool API - Runtime Environment",
    "APP_ENV=production",
    "GRPC_PORT=$GRPC_PORT",
    "DB_HOST=$RDS_HOST",
    "DB_PORT=5432",
    "DB_USER=$DB_USER",
    "DB_PASSWORD=$DB_PASSWORD",
    "DB_NAME=$DB_NAME",
    "DB_SSLMODE=require",
    "TLS_CERT=/etc/ubertool/certs/fullchain.pem",
    "TLS_KEY=/etc/ubertool/certs/privkey.pem"
)
$envContent | Set-Content "$BUILD_DIR\ubertool.env" -Encoding ASCII

# 3. Upload binary, env file, config, firebase key, and service unit to EC2
Write-Host "[3/6] Uploading files to EC2..."
scp -i $SSH_KEY_PATH -o StrictHostKeyChecking=no "$BUILD_DIR\$BINARY_NAME" "$BUILD_DIR\ubertool.env" "$PSScriptRoot\config.yaml" "$PROJECT_ROOT\config\firebase-admin-key.json" "$PSScriptRoot\ubertool-api.service" "ubuntu@${ELASTIC_IP}:/tmp/"

# 4. Install on EC2
Write-Host "[4/6] Installing binary and service on EC2..."
$installScript = "set -euo pipefail
sudo id -u ubertool > /dev/null 2>&1 || sudo useradd -r -s /bin/false ubertool
sudo mv /tmp/$BINARY_NAME /usr/local/bin/$BINARY_NAME
sudo chmod 755 /usr/local/bin/$BINARY_NAME
sudo chown ubertool:ubertool /usr/local/bin/$BINARY_NAME
sudo mkdir -p /etc/ubertool
sudo mv /tmp/ubertool.env /etc/ubertool/ubertool.env
sudo chown root:ubertool /etc/ubertool/ubertool.env
sudo chmod 640 /etc/ubertool/ubertool.env
sudo mv /tmp/config.yaml /etc/ubertool/config.yaml
sudo chown root:ubertool /etc/ubertool/config.yaml
sudo chmod 640 /etc/ubertool/config.yaml
sudo mv /tmp/firebase-admin-key.json /etc/ubertool/firebase-admin-key.json
sudo chown root:ubertool /etc/ubertool/firebase-admin-key.json
sudo chmod 640 /etc/ubertool/firebase-admin-key.json
sudo mkdir -p /var/ubertool/uploads/images /var/ubertool/uploads/thumbnails
sudo chown -R ubertool:ubertool /var/ubertool
sudo mkdir -p /var/lib/ubertool
sudo chown ubertool:ubertool /var/lib/ubertool
sudo mv /tmp/ubertool-api.service /etc/systemd/system/ubertool-api.service
sudo systemctl daemon-reload
sudo systemctl enable ubertool-api
echo 'Installation complete.'"

ssh -i $SSH_KEY_PATH -o StrictHostKeyChecking=no "ubuntu@$ELASTIC_IP" $installScript

# 5. Restart service
Write-Host "[5/6] Starting ubertool-api service..."
ssh -i $SSH_KEY_PATH -o StrictHostKeyChecking=no "ubuntu@$ELASTIC_IP" "sudo systemctl restart ubertool-api && sleep 2 && sudo systemctl status ubertool-api --no-pager"

# 6. Show recent logs
Write-Host "[6/6] Recent service logs:"
ssh -i $SSH_KEY_PATH -o StrictHostKeyChecking=no "ubuntu@$ELASTIC_IP" "sudo journalctl -u ubertool-api -n 20 --no-pager"

Write-Host ""
Write-Host "========================================"
Write-Host " Deployment Complete!"
Write-Host "========================================"
Write-Host ""
Write-Host "  gRPC endpoint : ${ELASTIC_IP}:${GRPC_PORT}"
Write-Host ""
Write-Host "  Stream logs:"
Write-Host "    ssh -i $SSH_KEY_PATH ubuntu@$ELASTIC_IP"
Write-Host "    sudo journalctl -u ubertool-api -f"
Write-Host ""
Write-Host "  To redeploy after code changes: .\04_deploy.ps1"
