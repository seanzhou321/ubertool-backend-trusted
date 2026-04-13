# =============================================================================
# 99_teardown.ps1
# Destroys all AWS resources created by 01_create_infra.ps1.
# USE WITH CAUTION - this is irreversible.
#
# Usage: .\99_teardown.ps1

$ErrorActionPreference = "Stop"

. "$PSScriptRoot\Load-Env.ps1"
Import-Env "$PSScriptRoot\config.env"
Import-Env "$PSScriptRoot\infra_state.env"

Write-Host "========================================"
Write-Host " Ubertool MVP - TEARDOWN"
Write-Host "========================================"
Write-Host ""
Write-Host "  This will permanently delete:"
Write-Host "    EC2 instance   : $INSTANCE_ID"
Write-Host "    Elastic IP     : $ELASTIC_IP ($ALLOC_ID)"
Write-Host "    RDS instance   : $APP_NAME-db"
Write-Host "    Security groups: $EC2_SG_ID, $RDS_SG_ID"
Write-Host ""
$confirm = Read-Host "  Type 'yes' to confirm"
if ($confirm -ne "yes") {
    Write-Host "Aborted."
    exit 1
}

Write-Host ""
Write-Host "[1/5] Disassociating and releasing Elastic IP..."
$assocId = $(aws ec2 describe-addresses --allocation-ids $ALLOC_ID --query "Addresses[0].AssociationId" --output text --region $AWS_REGION)
if ($assocId -and $assocId -ne "None") {
    aws ec2 disassociate-address --association-id $assocId --region $AWS_REGION | Out-Null
}
aws ec2 release-address --allocation-id $ALLOC_ID --region $AWS_REGION | Out-Null

Write-Host "[2/5] Terminating EC2 instance..."
aws ec2 terminate-instances --instance-ids $INSTANCE_ID --region $AWS_REGION | Out-Null
aws ec2 wait instance-terminated --instance-ids $INSTANCE_ID --region $AWS_REGION

Write-Host "[3/5] Deleting RDS instance (no final snapshot)..."
aws rds delete-db-instance --db-instance-identifier "$APP_NAME-db" --skip-final-snapshot --region $AWS_REGION | Out-Null
aws rds wait db-instance-deleted --db-instance-identifier "$APP_NAME-db" --region $AWS_REGION

Write-Host "[4/5] Deleting RDS subnet group..."
aws rds delete-db-subnet-group --db-subnet-group-name "$APP_NAME-subnet-group" --region $AWS_REGION | Out-Null

Write-Host "[5/5] Deleting security groups..."
aws ec2 delete-security-group --group-id $RDS_SG_ID --region $AWS_REGION | Out-Null
aws ec2 delete-security-group --group-id $EC2_SG_ID --region $AWS_REGION | Out-Null

Write-Host ""
Write-Host "========================================"
Write-Host " Teardown complete. All resources deleted."
Write-Host "========================================"
