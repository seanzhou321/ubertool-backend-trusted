# Podman teardown script for Trusted Ubertool Postgres
$CONTAINER_NAME = "ubertool-postgres"
$IMAGE_NAME = "ubertool-postgres-trusted"

Write-Host "Stopping and removing container..."
podman stop $CONTAINER_NAME
podman rm $CONTAINER_NAME

Write-Host "Removing image..."
podman rmi $IMAGE_NAME

Write-Host "Removing volume..."
podman volume rm ubertool-postgres-data

Write-Host "Teardown complete."
