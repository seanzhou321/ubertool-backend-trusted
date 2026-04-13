#!/usr/bin/env bash
# =============================================================================
# 02_init_db.sh
# Waits for RDS to be available, then applies the Ubertool schema via the
# EC2 instance (RDS is not publicly accessible).
#
# Usage:
#   ./02_init_db.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.env"
source "${SCRIPT_DIR}/infra_state.env"

SCHEMA_FILE="${SCRIPT_DIR}/../../podman/trusted-group/postgres/ubertool_schema_trusted.sql"

echo "========================================"
echo " Ubertool MVP — Database Initialization"
echo "========================================"

# ── 1. Wait for RDS to be available ──────────────────────────────────────────
echo "[1/4] Waiting for RDS instance to become available (may take 5–10 min)..."
aws rds wait db-instance-available \
  --db-instance-identifier "${APP_NAME}-db" \
  --region "${AWS_REGION}"

# ── 2. Fetch RDS endpoint ─────────────────────────────────────────────────────
echo "[2/4] Fetching RDS endpoint..."
RDS_HOST=$(aws rds describe-db-instances \
  --db-instance-identifier "${APP_NAME}-db" \
  --query "DBInstances[0].Endpoint.Address" \
  --output text \
  --region "${AWS_REGION}")
echo "      RDS Host: ${RDS_HOST}"

# Persist for later scripts
echo "RDS_HOST=${RDS_HOST}" >> "${SCRIPT_DIR}/infra_state.env"

# ── 3. Copy schema to EC2 ─────────────────────────────────────────────────────
echo "[3/4] Copying schema file to EC2..."
scp -i "${SSH_KEY_PATH}" \
    -o StrictHostKeyChecking=no \
    "${SCHEMA_FILE}" \
    "ubuntu@${ELASTIC_IP}:/tmp/ubertool_schema.sql"

# ── 4. Apply schema via EC2 ───────────────────────────────────────────────────
echo "[4/4] Applying schema on RDS via EC2..."
ssh -i "${SSH_KEY_PATH}" \
    -o StrictHostKeyChecking=no \
    "ubuntu@${ELASTIC_IP}" \
    "PGPASSWORD='${DB_PASSWORD}' psql \
      -h '${RDS_HOST}' \
      -U '${DB_USER}' \
      -d '${DB_NAME}' \
      -f /tmp/ubertool_schema.sql && rm /tmp/ubertool_schema.sql"

echo ""
echo "========================================"
echo " Database initialized successfully!"
echo "========================================"
echo ""
echo "  RDS Host: ${RDS_HOST}"
echo ""
echo "Next step: Run ./03_setup_tls.sh to obtain your TLS certificate."
