# Podman deployment script for Trusted Ubertool Postgres
$IMAGE_NAME = "ubertool-postgres-trusted"
$CONTAINER_NAME = "ubertool-postgres"
$DB_PASSWORD = "ubertool123"
$DB_USER = "ubertool_trusted"
$DB_NAME = "ubertool_db"

# Build the Podman image
Write-Host "Building Podman image..."
podman build -t $IMAGE_NAME -f Podmanfile .

# Create named volume for data persistence if it doesn't exist
Write-Host "Checking for named volume..."
$volumeExists = podman volume ls --format "{{.Name}}" | Select-String -Pattern "^ubertool-postgres-data$"
if (-not $volumeExists) {
    Write-Host "Creating named volume..."
    podman volume create ubertool-postgres-data
}

# Stop and remove existing container if it exists
$containerExists = podman ps -a --format "{{.Names}}" | Select-String -Pattern "^$CONTAINER_NAME$"
if ($containerExists) {
    Write-Host "Stopping and removing existing container..."
    podman stop $CONTAINER_NAME
    podman rm $CONTAINER_NAME
}

# Run the container
Write-Host "Running PostgreSQL container..."
podman run -d `
  --name $CONTAINER_NAME `
  -p 5432:5432 `
  -e POSTGRES_PASSWORD=$DB_PASSWORD `
  -e POSTGRES_USER=$DB_USER `
  -e POSTGRES_DB=$DB_NAME `
  -v ubertool-postgres-data:/var/lib/postgresql/data `
  $IMAGE_NAME

Write-Host ""
Write-Host "Database deployed successfully!"
Write-Host "PostgreSQL is running on localhost:5432"
Write-Host "Database: $DB_NAME"
Write-Host "User: $DB_USER"
Write-Host "Password: $DB_PASSWORD"
Write-Host ""
Write-Host "To connect:"
Write-Host "psql -h localhost -p 5432 -U $DB_USER -d $DB_NAME"
