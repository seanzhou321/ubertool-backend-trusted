# =============================================================================
# 02_init_db.ps1
# Waits for RDS to be available, then applies the Ubertool schema via
# the EC2 instance (RDS is not publicly accessible).
#
# Usage:
#   .\02_init_db.ps1
# =============================================================================

$ErrorActionPreference = "Stop"

. "$PSScriptRoot\Load-Env.ps1"
Import-Env "$PSScriptRoot\config.env"
Import-Env "$PSScriptRoot\infra_state.env"

$SCHEMA_FILE = Resolve-Path "$PSScriptRoot\..\..\podman\trusted-group\postgres\ubertool_schema_trusted.sql"

Write-Host "========================================"
Write-Host " Ubertool MVP - Database Initialization"
Write-Host "========================================"

# 1. Wait for RDS
Write-Host "[1/4] Waiting for RDS instance to become available (may take 5-10 min)..."
aws rds wait db-instance-available --db-instance-identifier "$APP_NAME-db" --region $AWS_REGION
Write-Host "      RDS is available."

# 2. Fetch RDS endpoint
Write-Host "[2/4] Fetching RDS endpoint..."
$RDS_HOST = $(aws rds describe-db-instances --db-instance-identifier "$APP_NAME-db" --query "DBInstances[0].Endpoint.Address" --output text --region $AWS_REGION)
Write-Host "      RDS Host: $RDS_HOST"

$stateAppend = Get-Content "$PSScriptRoot\infra_state.env"
$stateAppend += "RDS_HOST=$RDS_HOST"
$stateAppend | Set-Content "$PSScriptRoot\infra_state.env" -Encoding ASCII

# 3. Copy schema to EC2
Write-Host "[3/4] Copying schema file to EC2..."
scp -i $SSH_KEY_PATH -o StrictHostKeyChecking=no $SCHEMA_FILE "ubuntu@${ELASTIC_IP}:/tmp/ubertool_schema.sql"

# 4. Apply schema via psql on EC2
Write-Host "[4/4] Applying schema on RDS via EC2..."
$remoteCmd = "PGPASSWORD='$DB_PASSWORD' psql -h '$RDS_HOST' -U '$DB_USER' -d '$DB_NAME' -f /tmp/ubertool_schema.sql && rm /tmp/ubertool_schema.sql"
ssh -i $SSH_KEY_PATH -o StrictHostKeyChecking=no "ubuntu@$ELASTIC_IP" $remoteCmd

Write-Host ""
Write-Host "========================================"
Write-Host " Database initialized successfully!"
Write-Host "========================================"
Write-Host ""
Write-Host "  RDS Host: $RDS_HOST"
Write-Host ""
Write-Host "Next step: Run .\04_deploy.ps1"
