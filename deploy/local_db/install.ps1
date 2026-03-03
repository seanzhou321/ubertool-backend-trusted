# Install script for Ubertool Trusted Backend Database Schema
# This script populates the PostgreSQL database with the schema defined in ubertool_schema_trusted.sql

# Database connection parameters (from config.test.yaml)
$DB_HOST = "localhost"
$DB_PORT = "5454"
$DB_USER = "ubertool_trusted"
$DB_PASSWORD = "ubertool123"
$DB_NAME = "ubertool_db"

# Path to psql executable
$PSQL_PATH = "C:\Program Files\PostgreSQL\17\bin\psql.exe"

# Path to the schema file
$SCHEMA_FILE = "$PSScriptRoot\..\..\podman\trusted-group\postgres\ubertool_schema_trusted.sql"

# Set PGPASSWORD environment variable for psql
$env:PGPASSWORD = $DB_PASSWORD

# Run psql to execute the schema
Write-Host "Installing database schema..."
try {
    & $PSQL_PATH -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f $SCHEMA_FILE
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Schema installation completed successfully."
    } else {
        Write-Host "Error: Schema installation failed with exit code $LASTEXITCODE"
        exit $LASTEXITCODE
    }
} catch {
    Write-Host "Error: $($_.Exception.Message)"
    exit 1
} finally {
    # Clear the password from environment
    Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue
}