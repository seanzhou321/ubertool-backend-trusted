#!/usr/bin/env bash
# =============================================================================
# 03_setup_tls.sh
# Installs Certbot on the EC2 instance and obtains a Let's Encrypt TLS
# certificate for your domain.
#
# IMPORTANT: Your domain's A-record MUST already point to the Elastic IP
# before running this script. DNS propagation can take a few minutes.
#
# Why Let's Encrypt (not self-signed):
#   - Trusted by Android and iOS without any extra app configuration
#   - Free, auto-renewable, industry standard
#   - Required for gRPC TLS that works out-of-the-box on mobile clients
#
# Usage:
#   ./03_setup_tls.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.env"
source "${SCRIPT_DIR}/infra_state.env"

echo "========================================"
echo " Ubertool MVP — TLS Certificate Setup"
echo "========================================"
echo "  Domain : ${DOMAIN}"
echo "  Email  : ${CERTBOT_EMAIL}"
echo ""

# ── 1. Verify DNS resolution ──────────────────────────────────────────────────
echo "[1/3] Checking DNS resolution for ${DOMAIN}..."
RESOLVED_IP=$(dig +short "${DOMAIN}" | tail -n1 || true)
if [[ "${RESOLVED_IP}" != "${ELASTIC_IP}" ]]; then
  echo ""
  echo "  WARNING: ${DOMAIN} resolves to '${RESOLVED_IP}'"
  echo "           Expected Elastic IP: ${ELASTIC_IP}"
  echo ""
  echo "  Please update your domain's A-record to point to ${ELASTIC_IP}"
  echo "  and wait for DNS propagation before continuing."
  echo ""
  read -rp "  Continue anyway? (y/N): " confirm
  [[ "${confirm}" =~ ^[Yy]$ ]] || { echo "Aborted."; exit 1; }
fi

# ── 2. Install Certbot on EC2 ─────────────────────────────────────────────────
echo "[2/3] Installing Certbot and obtaining certificate on EC2..."
ssh -i "${SSH_KEY_PATH}" \
    -o StrictHostKeyChecking=no \
    "ubuntu@${ELASTIC_IP}" \
    bash -s <<'REMOTE'
set -euo pipefail
echo "  Installing certbot..."
sudo apt-get update -qq
sudo apt-get install -y -qq certbot

echo "  Stopping any process on port 80 temporarily..."
sudo fuser -k 80/tcp 2>/dev/null || true

echo "  Requesting certificate (standalone mode)..."
sudo certbot certonly \
  --standalone \
  --non-interactive \
  --agree-tos \
  --email "$CERTBOT_EMAIL_PLACEHOLDER" \
  --domain "$DOMAIN_PLACEHOLDER" \
  --cert-name ubertool

echo "  Setting up auto-renewal..."
# Certbot installs a systemd timer by default; verify it
sudo systemctl enable certbot.timer
sudo systemctl start certbot.timer

echo "  Granting app read access to certs..."
sudo mkdir -p /etc/ubertool/certs
sudo cp /etc/letsencrypt/live/ubertool/fullchain.pem /etc/ubertool/certs/
sudo cp /etc/letsencrypt/live/ubertool/privkey.pem   /etc/ubertool/certs/
sudo chmod 644 /etc/ubertool/certs/fullchain.pem
sudo chmod 640 /etc/ubertool/certs/privkey.pem
sudo chown root:ubertool /etc/ubertool/certs/privkey.pem 2>/dev/null || \
  sudo chown root:ubuntu /etc/ubertool/certs/privkey.pem

echo "  Certificate installed at /etc/ubertool/certs/"
REMOTE

# Replace placeholders with actual values via a second SSH pass
ssh -i "${SSH_KEY_PATH}" \
    -o StrictHostKeyChecking=no \
    "ubuntu@${ELASTIC_IP}" \
    "
set -euo pipefail
sudo fuser -k 80/tcp 2>/dev/null || true
sudo certbot certonly \
  --standalone \
  --non-interactive \
  --agree-tos \
  --email '${CERTBOT_EMAIL}' \
  -d '${DOMAIN}' \
  --cert-name ubertool

sudo mkdir -p /etc/ubertool/certs
sudo cp /etc/letsencrypt/live/ubertool/fullchain.pem /etc/ubertool/certs/
sudo cp /etc/letsencrypt/live/ubertool/privkey.pem   /etc/ubertool/certs/
sudo chmod 644 /etc/ubertool/certs/fullchain.pem
sudo chmod 640 /etc/ubertool/certs/privkey.pem
sudo chown root:ubuntu /etc/ubertool/certs/privkey.pem

# Add a renewal deploy hook to copy fresh certs after each renewal
sudo tee /etc/letsencrypt/renewal-hooks/deploy/copy_certs.sh > /dev/null <<'HOOK'
#!/bin/bash
cp /etc/letsencrypt/live/ubertool/fullchain.pem /etc/ubertool/certs/
cp /etc/letsencrypt/live/ubertool/privkey.pem   /etc/ubertool/certs/
chmod 644 /etc/ubertool/certs/fullchain.pem
chmod 640 /etc/ubertool/certs/privkey.pem
systemctl restart ubertool-api 2>/dev/null || true
HOOK
sudo chmod +x /etc/letsencrypt/renewal-hooks/deploy/copy_certs.sh

echo 'TLS setup complete.'
"

# ── 3. Summary ────────────────────────────────────────────────────────────────
echo "[3/3] TLS certificate successfully provisioned."
echo ""
echo "========================================"
echo " TLS Setup Complete!"
echo "========================================"
echo ""
echo "  Certificate : /etc/ubertool/certs/fullchain.pem"
echo "  Private key : /etc/ubertool/certs/privkey.pem"
echo "  Auto-renews : via systemd certbot.timer (every 12h check)"
echo ""
echo "  gRPC clients (Android & iOS) connect to:"
echo "    ${DOMAIN}:${GRPC_PORT}"
echo "  No custom CA needed — Let's Encrypt is trusted by all devices."
echo ""
echo "Next step: Run ./04_deploy.sh to build and deploy the Go binary."
