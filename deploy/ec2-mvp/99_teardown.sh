#!/usr/bin/env bash
# =============================================================================
# 99_teardown.sh
# Destroys all AWS resources created by 01_create_infra.sh.
# USE WITH CAUTION — this is irreversible.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.env"
source "${SCRIPT_DIR}/infra_state.env"

echo "========================================"
echo " Ubertool MVP — TEARDOWN"
echo "========================================"
echo ""
echo "  This will permanently delete:"
echo "    EC2 instance  : ${INSTANCE_ID}"
echo "    Elastic IP    : ${ELASTIC_IP} (${ALLOC_ID})"
echo "    RDS instance  : ${APP_NAME}-db"
echo "    Security groups: ${EC2_SG_ID}, ${RDS_SG_ID}"
echo ""
read -rp "  Type 'yes' to confirm: " confirm
[[ "${confirm}" == "yes" ]] || { echo "Aborted."; exit 1; }

echo ""
echo "[1/5] Disassociating and releasing Elastic IP..."
ASSOC_ID=$(aws ec2 describe-addresses \
  --allocation-ids "${ALLOC_ID}" \
  --query "Addresses[0].AssociationId" \
  --output text \
  --region "${AWS_REGION}" 2>/dev/null || echo "None")
[[ "${ASSOC_ID}" != "None" && -n "${ASSOC_ID}" ]] && \
  aws ec2 disassociate-address --association-id "${ASSOC_ID}" --region "${AWS_REGION}"
aws ec2 release-address --allocation-id "${ALLOC_ID}" --region "${AWS_REGION}"

echo "[2/5] Terminating EC2 instance..."
aws ec2 terminate-instances --instance-ids "${INSTANCE_ID}" --region "${AWS_REGION}" > /dev/null
aws ec2 wait instance-terminated --instance-ids "${INSTANCE_ID}" --region "${AWS_REGION}"

echo "[3/5] Deleting RDS instance (no final snapshot)..."
aws rds delete-db-instance \
  --db-instance-identifier "${APP_NAME}-db" \
  --skip-final-snapshot \
  --region "${AWS_REGION}" > /dev/null
aws rds wait db-instance-deleted \
  --db-instance-identifier "${APP_NAME}-db" \
  --region "${AWS_REGION}"

echo "[4/5] Deleting RDS subnet group..."
aws rds delete-db-subnet-group \
  --db-subnet-group-name "${APP_NAME}-subnet-group" \
  --region "${AWS_REGION}"

echo "[5/5] Deleting security groups..."
aws ec2 delete-security-group --group-id "${RDS_SG_ID}" --region "${AWS_REGION}"
aws ec2 delete-security-group --group-id "${EC2_SG_ID}" --region "${AWS_REGION}"

echo ""
echo "========================================"
echo " Teardown complete. All resources deleted."
echo "========================================"
