package e2e

import (
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/require"
)

func TestDebugToolStatus(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "")
	defer client.Close()

	imageClient := pb.NewImageStorageServiceClient(client.Conn())

	ownerID := db.CreateTestUser("e2e-test-debug-owner@test.com", "Debug Owner")
	nonOwnerID := db.CreateTestUser("e2e-test-debug-nonowner@test.com", "Debug NonOwner")
	toolID := db.CreateTestTool(ownerID, "Debug Tool", 1000)

	// Check initial status
	var status string
	err := db.QueryRow("SELECT status FROM tools WHERE id = $1", toolID).Scan(&status)
	require.NoError(t, err)
	t.Logf("Initial tool status: %s", status)

	// First call - should succeed
	ctx, cancel := ContextWithUserIDAndTimeout(nonOwnerID, 5*time.Second)
	defer cancel()

	resp, err := imageClient.GetToolImages(ctx, &pb.GetToolImagesRequest{ToolId: toolID})
	require.NoError(t, err, "First call should succeed")
	t.Logf("First call succeeded: %d images", len(resp.Images))

	// Update to RENTED
	_, err = db.Exec("UPDATE tools SET status = 'RENTED' WHERE id = $1", toolID)
	require.NoError(t, err)

	// Check status after update
	err = db.QueryRow("SELECT status FROM tools WHERE id = $1", toolID).Scan(&status)
	require.NoError(t, err)
	t.Logf("Tool status after UPDATE: %s", status)

	// Second call - should fail
	_, err = imageClient.GetToolImages(ctx, &pb.GetToolImagesRequest{ToolId: toolID})
	if err != nil {
		t.Logf("Second call failed as expected: %v", err)
	} else {
		t.Logf("Second call succeeded UNEXPECTEDLY: %d images", len(resp.Images))
	}
}
