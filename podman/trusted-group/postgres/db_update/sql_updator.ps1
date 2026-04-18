# Applies dbupdate.sql to the Postgres container deployed by postgres/install.ps1
$CONTAINER_NAME = "ubertool-postgres"
$DB_USER        = "ubertool_trusted"
$DB_NAME        = "ubertool_db"
$SQL_FILE       = "dbupdate.sql"

# Verify the container is running
$running = podman ps --format "{{.Names}}" | Select-String -Pattern "^$CONTAINER_NAME$"
if (-not $running) {
    Write-Error "Container '$CONTAINER_NAME' is not running. Start it with postgres/install.ps1 first."
    exit 1
}

# Verify the SQL file exists next to this script
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$sqlPath   = Join-Path $scriptDir $SQL_FILE
if (-not (Test-Path $sqlPath)) {
    Write-Error "SQL file not found: $sqlPath"
    exit 1
}

Write-Host "Executing $SQL_FILE on database '$DB_NAME'..."
Get-Content $sqlPath -Raw | podman exec -i $CONTAINER_NAME psql -U $DB_USER -d $DB_NAME
if ($LASTEXITCODE -ne 0) {
    Write-Error "SQL execution failed."
    exit 1
}

Write-Host ""
Write-Host "Database update applied successfully."
