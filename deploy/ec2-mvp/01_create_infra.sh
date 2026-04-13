#!/usr/bin/env bash
# =============================================================================
# 01_create_infra.sh
# Creates EC2 (t3.micro), RDS PostgreSQL (db.t3.micro), Security Groups,
# and allocates an Elastic IP for the Ubertool backend MVP.
#
# Prerequisites:
#   - AWS CLI v2 installed and configured  (`aws configure`)
#   - A key pair already created in AWS Console (or create one below)
#   - Copy config.env.template -> config.env and fill in your values
#
# Usage:
#   chmod +x 01_create_infra.sh
#   ./01_create_infra.sh
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/config.env"

echo "========================================"
echo " Ubertool MVP — AWS Infrastructure Setup"
echo "========================================"
echo "Region  : ${AWS_REGION}"
echo "App Name: ${APP_NAME}"
echo ""

# ── 1. VPC lookup (uses your default VPC) ────────────────────────────────────
echo "[1/9] Fetching default VPC..."
VPC_ID=$(aws ec2 describe-vpcs \
  --filters "Name=isDefault,Values=true" \
  --query "Vpcs[0].VpcId" \
  --output text \
  --region "${AWS_REGION}")
echo "      VPC ID: ${VPC_ID}"

# Fetch two subnets in different AZs (required for RDS subnet group)
SUBNET_IDS=$(aws ec2 describe-subnets \
  --filters "Name=vpc-id,Values=${VPC_ID}" \
  --query "Subnets[*].SubnetId" \
  --output text \
  --region "${AWS_REGION}")
SUBNET_ARRAY=($SUBNET_IDS)
SUBNET_1="${SUBNET_ARRAY[0]}"
SUBNET_2="${SUBNET_ARRAY[1]}"
EC2_SUBNET="${SUBNET_ARRAY[0]}"
echo "      Subnets: ${SUBNET_1}, ${SUBNET_2}"

# ── 2. Security Group — EC2 ───────────────────────────────────────────────────
echo "[2/9] Creating EC2 security group..."
EC2_SG_ID=$(aws ec2 create-security-group \
  --group-name "${APP_NAME}-ec2-sg" \
  --description "Ubertool EC2: SSH + gRPC TLS" \
  --vpc-id "${VPC_ID}" \
  --query "GroupId" \
  --output text \
  --region "${AWS_REGION}")
echo "      EC2 SG: ${EC2_SG_ID}"

# SSH (restrict to your IP in production)
aws ec2 authorize-security-group-ingress \
  --group-id "${EC2_SG_ID}" \
  --protocol tcp --port 22 --cidr "0.0.0.0/0" \
  --region "${AWS_REGION}"

# gRPC over TLS
aws ec2 authorize-security-group-ingress \
  --group-id "${EC2_SG_ID}" \
  --protocol tcp --port "${GRPC_PORT}" --cidr "0.0.0.0/0" \
  --region "${AWS_REGION}"

# HTTPS — needed for Let's Encrypt ACME challenge
aws ec2 authorize-security-group-ingress \
  --group-id "${EC2_SG_ID}" \
  --protocol tcp --port 80 --cidr "0.0.0.0/0" \
  --region "${AWS_REGION}"

aws ec2 authorize-security-group-ingress \
  --group-id "${EC2_SG_ID}" \
  --protocol tcp --port 443 --cidr "0.0.0.0/0" \
  --region "${AWS_REGION}"

# ── 3. Security Group — RDS ───────────────────────────────────────────────────
echo "[3/9] Creating RDS security group..."
RDS_SG_ID=$(aws ec2 create-security-group \
  --group-name "${APP_NAME}-rds-sg" \
  --description "Ubertool RDS: PostgreSQL from EC2 only" \
  --vpc-id "${VPC_ID}" \
  --query "GroupId" \
  --output text \
  --region "${AWS_REGION}")
echo "      RDS SG: ${RDS_SG_ID}"

# Only allow Postgres connections from the EC2 security group
aws ec2 authorize-security-group-ingress \
  --group-id "${RDS_SG_ID}" \
  --protocol tcp --port 5432 \
  --source-group "${EC2_SG_ID}" \
  --region "${AWS_REGION}"

# ── 4. RDS Subnet Group ───────────────────────────────────────────────────────
echo "[4/9] Creating RDS subnet group..."
aws rds create-db-subnet-group \
  --db-subnet-group-name "${APP_NAME}-subnet-group" \
  --db-subnet-group-description "Ubertool RDS subnet group" \
  --subnet-ids "${SUBNET_1}" "${SUBNET_2}" \
  --region "${AWS_REGION}" > /dev/null
echo "      Subnet group created."

# ── 5. RDS PostgreSQL Instance ────────────────────────────────────────────────
echo "[5/9] Creating RDS db.t3.micro PostgreSQL instance (this takes ~5 min)..."
aws rds create-db-instance \
  --db-instance-identifier "${APP_NAME}-db" \
  --db-instance-class db.t3.micro \
  --engine postgres \
  --engine-version "15.7" \
  --master-username "${DB_USER}" \
  --master-user-password "${DB_PASSWORD}" \
  --db-name "${DB_NAME}" \
  --allocated-storage 20 \
  --storage-type gp2 \
  --no-publicly-accessible \
  --vpc-security-group-ids "${RDS_SG_ID}" \
  --db-subnet-group-name "${APP_NAME}-subnet-group" \
  --backup-retention-period 7 \
  --no-multi-az \
  --region "${AWS_REGION}" > /dev/null
echo "      RDS instance creation initiated."

# ── 6. EC2 Instance ───────────────────────────────────────────────────────────
echo "[6/9] Launching EC2 t3.micro instance..."
INSTANCE_ID=$(aws ec2 run-instances \
  --image-id "${AMI_ID}" \
  --instance-type t3.micro \
  --key-name "${KEY_PAIR_NAME}" \
  --security-group-ids "${EC2_SG_ID}" \
  --subnet-id "${EC2_SUBNET}" \
  --user-data "file://${SCRIPT_DIR}/ec2_userdata.sh" \
  --tag-specifications \
    "ResourceType=instance,Tags=[{Key=Name,Value=${APP_NAME}-api}]" \
  --query "Instances[0].InstanceId" \
  --output text \
  --region "${AWS_REGION}")
echo "      Instance ID: ${INSTANCE_ID}"

# ── 7. Wait for EC2 to be running ─────────────────────────────────────────────
echo "[7/9] Waiting for EC2 instance to reach 'running' state..."
aws ec2 wait instance-running \
  --instance-ids "${INSTANCE_ID}" \
  --region "${AWS_REGION}"
echo "      EC2 instance is running."

# ── 8. Elastic IP ─────────────────────────────────────────────────────────────
echo "[8/9] Allocating and associating Elastic IP..."
ALLOC_ID=$(aws ec2 allocate-address \
  --domain vpc \
  --query "AllocationId" \
  --output text \
  --region "${AWS_REGION}")

aws ec2 associate-address \
  --instance-id "${INSTANCE_ID}" \
  --allocation-id "${ALLOC_ID}" \
  --region "${AWS_REGION}" > /dev/null

ELASTIC_IP=$(aws ec2 describe-addresses \
  --allocation-ids "${ALLOC_ID}" \
  --query "Addresses[0].PublicIp" \
  --output text \
  --region "${AWS_REGION}")
echo "      Elastic IP: ${ELASTIC_IP}"

# ── 9. Save state file ────────────────────────────────────────────────────────
echo "[9/9] Writing infra state to infra_state.env..."
cat > "${SCRIPT_DIR}/infra_state.env" <<EOF
# Auto-generated by 01_create_infra.sh — do not edit manually
INSTANCE_ID=${INSTANCE_ID}
ELASTIC_IP=${ELASTIC_IP}
ALLOC_ID=${ALLOC_ID}
EC2_SG_ID=${EC2_SG_ID}
RDS_SG_ID=${RDS_SG_ID}
VPC_ID=${VPC_ID}
EOF

echo ""
echo "========================================"
echo " Infrastructure created successfully!"
echo "========================================"
echo ""
echo "  EC2 Instance ID : ${INSTANCE_ID}"
echo "  Elastic IP      : ${ELASTIC_IP}"
echo "  RDS identifier  : ${APP_NAME}-db (still initializing)"
echo ""
echo "Next steps:"
echo "  1. Point your domain's A-record to: ${ELASTIC_IP}"
echo "  2. Wait ~5 min for RDS to finish, then run: ./02_init_db.sh"
echo "  3. SSH in to verify EC2: ssh -i ~/.ssh/${KEY_PAIR_NAME}.pem ubuntu@${ELASTIC_IP}"
echo ""
echo "  RDS status: aws rds describe-db-instances \\"
echo "    --db-instance-identifier ${APP_NAME}-db \\"
echo "    --query 'DBInstances[0].DBInstanceStatus' --output text"
