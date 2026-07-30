package e2e

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"
	"github.com/stretchr/testify/require"
)

func TestDebugToolStatus3(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "")
	defer client.Close()

	imageClient := pb.NewImageStorageServiceClient(client.Conn())

	cfg := loadConfig(t)
	uploadDir := cfg.Storage.UploadDir
	if !filepath.IsAbs(uploadDir) {
		uploadDir = filepath.Join("..", "..", uploadDir)
		uploadDir, _ = filepath.Abs(uploadDir)
	}

	unique := time.Now().UnixNano()
	ownerID := db.CreateTestUser(fmt.Sprintf("e2e-debug3-owner-%d@test.com", unique), "Debug Owner")
	nonOwnerID := db.CreateTestUser(fmt.Sprintf("e2e-debug3-nonowner-%d@test.com", unique), "Debug NonOwner")
	toolID := db.CreateTestTool(ownerID, "Debug Tool", 1000)

	// Insert test image
	_, err := db.Exec(`
		INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, is_primary, display_order, status, user_id)
		VALUES ($1, 'debug.jpg', $2, $3, 1024, 'image/jpeg', true, 0, 'CONFIRMED', $4)
	`, toolID, filepath.Join(uploadDir, "debug.jpg"), filepath.Join(uploadDir, "thumb_debug.jpg"), ownerID)
	require.NoError(t, err)

	// First call - tool is AVAILABLE, non-owner should have access
	ctx, cancel := ContextWithUserIDAndTimeout(nonOwnerID, 5*time.Second)
	defer cancel()

	resp, err := imageClient.GetToolImages(ctx, &pb.GetToolImagesRequest{ToolId: toolID})
	require.NoError(t, err, "First call should succeed for AVAILABLE tool")
	t.Logf("First call: %d images, error=%v", len(resp.Images), err)

	// Update to RENTED
	_, err = db.Exec("UPDATE tools SET status = 'RENTED' WHERE id = $1", toolID)
	require.NoError(t, err)

	// Verify status
	var status string
	err = db.QueryRow("SELECT status FROM tools WHERE id = $1", toolID).Scan(&status)
	require.NoError(t, err)
	t.Logf("Tool status after UPDATE: %s", status)

	// Second call - tool is RENTED, non-owner should be rejected
	ctx2, cancel2 := ContextWithUserIDAndTimeout(nonOwnerID, 5*time.Second)
	defer cancel2()

	resp2, err := imageClient.GetToolImages(ctx2, &pb.GetToolImagesRequest{ToolId: toolID})
	t.Logf("Second call: %d images, error=%v", len(resp2.Images), err)
	
	// Also test with owner - should still work
	ctx3, cancel3 := ContextWithUserIDAndTimeout(ownerID, 5*time.Second)
	defer cancel3()
	resp3, err := imageClient.GetToolImages(ctx3, &pb.GetToolImagesRequest{ToolId: toolID})
	t.Logf("Third call (owner): %d images, error=%v", len(resp3.Images), err)
}
