# Podman teardown script for the Ubertool Trusted Backend gRPC server.
# Stops/removes the container and image. Does NOT touch ubertool-postgres —
# use podman/trusted-group/postgres/teardown.ps1 (make db-teardown) for that.
$CONTAINER_NAME = "ubertool-service"
$IMAGE_NAME = "ubertool-service"

Write-Host "Teardown: Stopping and removing container '$CONTAINER_NAME'..."
podman stop $CONTAINER_NAME 2>&1 | Out-Null
podman rm -f $CONTAINER_NAME 2>&1 | Out-Null
if (podman container exists $CONTAINER_NAME) {
    Write-Warning "Failed to remove container $CONTAINER_NAME. It might still exist."
} else {
    Write-Host "Container removed (or didn't exist)."
}

Write-Host "Teardown: Removing image '$IMAGE_NAME'..."
podman rmi -f $IMAGE_NAME 2>&1 | Out-Null
podman rmi -f "localhost/$IMAGE_NAME" 2>&1 | Out-Null
$finalCheck = podman images --format "{{.Repository}}" | Select-String "$IMAGE_NAME"
if ($finalCheck) {
    Write-Warning "Image matching '$IMAGE_NAME' still found: $finalCheck"
} else {
    Write-Host "Image removed (or didn't exist)."
}

Write-Host "Teardown complete."
