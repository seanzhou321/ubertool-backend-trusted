# Teardown script for Ubertool Trusted Backend Database Schema
# This script removes all tables, functions, and triggers from the database

# Database connection parameters (from config.test.yaml)
$DB_HOST = "localhost"
$DB_PORT = "5454"
$DB_USER = "ubertool_trusted"
$DB_PASSWORD = "ubertool123"
$DB_NAME = "ubertool_db"

# Path to psql executable
$PSQL_PATH = "C:\Program Files\PostgreSQL\17\bin\psql.exe"

# Set PGPASSWORD environment variable for psql
$env:PGPASSWORD = $DB_PASSWORD

# SQL commands to drop everything
$DROP_SQL = @"
-- Drop trigger and function first
DROP TRIGGER IF EXISTS trigger_update_balance ON ledger_transactions;
DROP FUNCTION IF EXISTS update_user_balance();

-- Drop tables in reverse order to handle foreign keys
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS ledger_transactions CASCADE;
DROP TABLE IF EXISTS rentals CASCADE;
DROP TABLE IF EXISTS tool_images CASCADE;
DROP TABLE IF EXISTS tools CASCADE;
DROP TABLE IF EXISTS join_requests CASCADE;
DROP TABLE IF EXISTS invitations CASCADE;
DROP TABLE IF EXISTS users_orgs CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS orgs CASCADE;
"@

# Run psql to execute the drop commands
Write-Host "Tearing down database schema..."
try {
    $DROP_SQL | & $PSQL_PATH -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -q
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Schema teardown completed successfully."
    } else {
        Write-Host "Error: Schema teardown failed with exit code $LASTEXITCODE"
        exit $LASTEXITCODE
    }
} catch {
    Write-Host "Error: $($_.Exception.Message)"
    exit 1
} finally {
    # Clear the password from environment
    Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue
}