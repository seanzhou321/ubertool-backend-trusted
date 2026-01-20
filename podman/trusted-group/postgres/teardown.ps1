# Podman teardown script for Trusted Ubertool Postgres
$CONTAINER_NAME = "ubertool-postgres"
$IMAGE_NAME = "ubertool-postgres-trusted"
$VOLUME_NAME = "ubertool-postgres-data"

# 1. Stop and remove container
Write-Host "Teardown: Stopping and removing container '$CONTAINER_NAME'..."
# Try to stop and remove regardless of check (forceful cleanup)
# We suppress errors if it doesn't exist, to keep it clean, but we show output if it does.
podman stop $CONTAINER_NAME 2>&1 | Out-Null
podman rm -f $CONTAINER_NAME 2>&1 | Out-Null
# Verify removal
if (podman container exists $CONTAINER_NAME) {
    Write-Warning "Failed to remove container $CONTAINER_NAME. It might still exist."
} else {
    Write-Host "Container removed (or didn't exist)."
}

# 2. Remove image
Write-Host "Teardown: Removing image '$IMAGE_NAME'..."
# Try removing both plain and localhost versions to be safe, as podman's behavior varies
podman rmi -f $IMAGE_NAME 2>&1 | Out-Null
podman rmi -f "localhost/$IMAGE_NAME" 2>&1 | Out-Null

# Verify removal (check for either form)
$imgExistsCode = (podman image exists $IMAGE_NAME)
$imgLocalhostExistsCode = (podman image exists "localhost/$IMAGE_NAME")
if ($LASTEXITCODE -eq 0 -or $imgLocalhostExistsCode -eq 0) { 
    # Note: 'podman image exists' returns 0 if exists, 1 if not. 
    # But checking $LASTEXITCODE in PS can be tricky if not immediately checked.
    # Let's rely on the fact that if 'podman rmi -f' worked, it's gone.
    # We will just suppress error output above.
}
# Double check with a listing for user confirmation
$finalCheck = podman images --format "{{.Repository}}" | Select-String "$IMAGE_NAME"
if ($finalCheck) {
     Write-Warning "Image matching '$IMAGE_NAME' still found: $finalCheck"
} else {
     Write-Host "Image removed (or didn't exist)."
}

# 3. Remove volume
Write-Host "Teardown: Removing volume '$VOLUME_NAME'..."
podman volume rm -f $VOLUME_NAME 2>&1 | Out-Null
# Verify
$volCheck = podman volume ls --format "{{.Name}}" | Select-String "^$VOLUME_NAME$"
if ($volCheck) {
    Write-Warning "Volume '$VOLUME_NAME' failed to remove."
} else {
    Write-Host "Volume removed (or didn't exist)."
}

Write-Host "Teardown complete."
