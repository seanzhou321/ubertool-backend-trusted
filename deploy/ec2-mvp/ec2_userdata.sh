#!/usr/bin/env bash
# =============================================================================
# ec2_userdata.sh
# Runs automatically on EC2 first boot (passed as --user-data in 01_create_infra.sh).
# Installs runtime dependencies and prepares the filesystem layout.
# =============================================================================

set -euo pipefail

exec > /var/log/ubertool-init.log 2>&1
echo "=== Ubertool EC2 Bootstrap: $(date) ==="

# ── System updates ────────────────────────────────────────────────────────────
apt-get update -qq
apt-get upgrade -y -qq
apt-get install -y -qq \
  curl \
  wget \
  unzip \
  git \
  postgresql-client \
  dnsutils \
  htop \
  fail2ban

# ── Directory layout ──────────────────────────────────────────────────────────
mkdir -p /etc/ubertool/certs
mkdir -p /var/log/ubertool
mkdir -p /var/lib/ubertool

# ── App user ──────────────────────────────────────────────────────────────────
id -u ubertool &>/dev/null || useradd -r -s /bin/false ubertool
chown ubertool:ubertool /var/log/ubertool
chown ubertool:ubertool /var/lib/ubertool

# ── fail2ban (basic SSH protection) ──────────────────────────────────────────
systemctl enable fail2ban
systemctl start fail2ban

echo "=== Bootstrap complete: $(date) ==="
