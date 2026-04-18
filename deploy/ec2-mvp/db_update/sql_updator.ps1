# =============================================================================
# db_update/sql_updator.ps1
# Applies dbupdate.sql to the RDS database via the EC2 bastion.
# RDS is not publicly accessible, so psql runs on the EC2 instance.
#
# Usage:
#   .\db_update\sql_updator.ps1
# =============================================================================

$ErrorActionPreference = "Stop"

. "$PSScriptRoot\..\Load-Env.ps1"
Import-Env "$PSScriptRoot\..\config.env"
Import-Env "$PSScriptRoot\..\infra_state.env"

$SQL_FILE     = "$PSScriptRoot\dbupdate.sql"
$REMOTE_PATH  = "/tmp/dbupdate.sql"

if (-not (Test-Path $SQL_FILE)) {
    Write-Error "SQL file not found: $SQL_FILE"
    exit 1
}

Write-Host "========================================"
Write-Host " Ubertool MVP - Database Update"
Write-Host "========================================"
Write-Host "  EC2    : ubuntu@$ELASTIC_IP"
Write-Host "  RDS    : $RDS_HOST"
Write-Host "  DB     : $DB_NAME"
Write-Host ""

# 1. Copy SQL file to EC2
Write-Host "[1/2] Copying dbupdate.sql to EC2..."
scp -i $SSH_KEY_PATH -o StrictHostKeyChecking=no $SQL_FILE "ubuntu@${ELASTIC_IP}:$REMOTE_PATH"
if ($LASTEXITCODE -ne 0) {
    Write-Error "scp failed."
    exit 1
}

# 2. Apply update via psql on EC2, then remove the file
Write-Host "[2/2] Applying update on RDS via EC2..."
$remoteCmd = "PGPASSWORD='$DB_PASSWORD' psql -h '$RDS_HOST' -U '$DB_USER' -d '$DB_NAME' -f $REMOTE_PATH && rm $REMOTE_PATH"
ssh -i $SSH_KEY_PATH -o StrictHostKeyChecking=no "ubuntu@$ELASTIC_IP" $remoteCmd
if ($LASTEXITCODE -ne 0) {
    Write-Error "psql execution failed."
    exit 1
}

Write-Host ""
Write-Host "========================================"
Write-Host " Database update applied successfully!"
Write-Host "========================================"
