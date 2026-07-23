package e2e

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestPNG encodes a genuine, decodable 1x1 PNG. The pipeline this feeds into
// (ConfirmImageUpload's async thumbnail generation, FR-006) actually decodes the bytes with
// image.Decode, so — unlike a hand-rolled placeholder byte sequence — this must be real, valid
// PNG data or that decode step fails silently (the goroutine only logs and returns).
func newTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 200, G: 50, B: 50, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestImageStorageService_E2E(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "")
	defer client.Close()

	imageClient := pb.NewImageStorageServiceClient(client.Conn())

	t.Run("Upload and Download Image via Presigned URL", func(t *testing.T) {
		// Setup: Create user and tool
		userID := db.CreateTestUser("e2e-test-image-user@test.com", "Image User")
		orgID := db.CreateTestOrg("")
		db.AddUserToOrg(userID, orgID, "MEMBER", "ACTIVE", 0)
		toolID := db.CreateTestTool(userID, "Image Test Tool", 1000)

		// Create a genuine, decodable 1x1 PNG (needed so the real async thumbnail-generation
		// pipeline, exercised below, can successfully decode it — see FR-006).
		testImageData := newTestPNG(t)

		// Step 1: Request upload URL
		ctx1, cancel1 := ContextWithUserIDAndTimeout(userID, 10*time.Second)
		defer cancel1()

		uploadReq := &pb.GetUploadUrlRequest{
			ToolId:      toolID,
			Filename:    "test-image.png",
			ContentType: "image/png",
			IsPrimary:   true,
		}

		uploadResp, err := imageClient.GetUploadUrl(ctx1, uploadReq)
		require.NoError(t, err)
		require.NotEmpty(t, uploadResp.UploadUrl)
		require.NotZero(t, uploadResp.ImageId)
		imageID := uploadResp.ImageId

		// Step 2: Upload file to presigned URL
		err = uploadFileToURL(uploadResp.UploadUrl, "test-image.png", "image/png", testImageData)
		require.NoError(t, err)

		// Step 3: Confirm upload
		ctx2, cancel2 := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel2()

		confirmReq := &pb.ConfirmImageUploadRequest{
			ImageId:  imageID,
			ToolId:   toolID,
			FileSize: int64(len(testImageData)),
		}

		confirmResp, err := imageClient.ConfirmImageUpload(ctx2, confirmReq)
		require.NoError(t, err)
		assert.True(t, confirmResp.Success)

		// Verify: Image record in database is CONFIRMED
		var fileName, status string
		err = db.QueryRow("SELECT file_name, status FROM tool_images WHERE id = $1", imageID).Scan(&fileName, &status)
		assert.NoError(t, err)
		assert.Equal(t, "test-image.png", fileName)
		assert.Equal(t, "CONFIRMED", status)

		// Verify FR-006: the real (async) thumbnail-generation pipeline actually ran to
		// completion — thumbnail_path must be populated, and the file it points to must exist
		// on disk and be a real JPEG (not a placeholder). ConfirmImageUpload kicks this off in a
		// background goroutine, so poll briefly rather than asserting immediately.
		cfg := loadConfig(t)
		uploadDir := cfg.Storage.UploadDir
		if !filepath.IsAbs(uploadDir) {
			uploadDir = filepath.Join("..", "..", uploadDir)
			uploadDir, _ = filepath.Abs(uploadDir)
		}
		var thumbnailPath string
		require.Eventually(t, func() bool {
			err := db.QueryRow("SELECT thumbnail_path FROM tool_images WHERE id = $1", imageID).Scan(&thumbnailPath)
			return err == nil && thumbnailPath != ""
		}, 5*time.Second, 100*time.Millisecond, "thumbnail_path must be populated once the async pipeline completes")

		fullThumbPath := filepath.Join(uploadDir, "images", thumbnailPath)
		thumbBytes, err := os.ReadFile(fullThumbPath)
		require.NoError(t, err, "the generated thumbnail file must actually exist on disk")
		require.Greater(t, len(thumbBytes), 3)
		assert.Equal(t, []byte{0xFF, 0xD8, 0xFF}, thumbBytes[:3], "generated thumbnail must be a real JPEG, not a placeholder")

		// Step 4: Download image via presigned URL
		ctx3, cancel3 := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel3()

		downloadReq := &pb.GetDownloadUrlRequest{
			ImageId:     imageID,
			ToolId:      toolID,
			IsThumbnail: false,
		}

		downloadResp, err := imageClient.GetDownloadUrl(ctx3, downloadReq)
		require.NoError(t, err)
		require.NotEmpty(t, downloadResp.DownloadUrl)

		// Download file from presigned URL
		downloadedData, err := downloadFileFromURL(downloadResp.DownloadUrl)
		require.NoError(t, err)

		// Verify: Downloaded data matches uploaded data
		assert.Equal(t, testImageData, downloadedData)
	})

	t.Run("GetToolImages", func(t *testing.T) {
		// Setup: Create user, tool, and multiple images
		userID := db.CreateTestUser("e2e-test-getimages@test.com", "Get Images User")
		toolID := db.CreateTestTool(userID, "Multi Image Tool", 1000)

		// Get upload directory from config and convert to absolute path
		cfg := loadConfig(t)
		uploadDir := cfg.Storage.UploadDir
		if !filepath.IsAbs(uploadDir) {
			uploadDir = filepath.Join("..", "..", uploadDir)
			uploadDir, _ = filepath.Abs(uploadDir)
		}

		// Create test images in database
		_, err := db.Exec(`
			INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, is_primary, display_order, status, user_id)
			VALUES 
				($1, 'image1.jpg', $2, $3, 1024, 'image/jpeg', true, 0, 'CONFIRMED', $4),
				($1, 'image2.jpg', $5, $6, 2048, 'image/jpeg', false, 1, 'CONFIRMED', $4),
				($1, 'image3.png', $7, $8, 1536, 'image/png', false, 2, 'CONFIRMED', $4)
		`, toolID,
			filepath.Join(uploadDir, "image1.jpg"), filepath.Join(uploadDir, "thumb1.jpg"), userID,
			filepath.Join(uploadDir, "image2.jpg"), filepath.Join(uploadDir, "thumb2.jpg"),
			filepath.Join(uploadDir, "image3.png"), filepath.Join(uploadDir, "thumb3.png"))
		require.NoError(t, err)

		// Test: Get tool images
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.GetToolImagesRequest{
			ToolId: toolID,
		}

		resp, err := imageClient.GetToolImages(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, 3, len(resp.Images))

		// Verify: Primary image is first
		assert.True(t, resp.Images[0].IsPrimary)
		assert.Equal(t, "image1.jpg", resp.Images[0].FileName)
	})

	// FR-007 (specs/006-tools-image-storage): GetToolImages does not enforce the same
	// owner-or-AVAILABLE access rule as GetDownloadUrl — internal/service/image_storage.go's
	// GetToolImages takes no userID parameter at all. This documents the current (lack of)
	// access boundary as a regression baseline: if access control is ever added here, this test
	// will fail, correctly signaling the as-built behavior changed on purpose (see FR-007's own
	// wording — it explicitly describes a target-not-current state, not a required fix).
	t.Run("GetToolImages does not restrict access to the owner (documents current non-enforcement)", func(t *testing.T) {
		ownerID := db.CreateTestUser("e2e-test-getimages-owner@test.com", "Images Owner")
		nonOwnerID := db.CreateTestUser("e2e-test-getimages-nonowner@test.com", "Non Owner Viewer")
		toolID := db.CreateTestTool(ownerID, "Non-Owner-Viewable Tool", 1000)

		cfg := loadConfig(t)
		uploadDir := cfg.Storage.UploadDir
		if !filepath.IsAbs(uploadDir) {
			uploadDir = filepath.Join("..", "..", uploadDir)
			uploadDir, _ = filepath.Abs(uploadDir)
		}

		_, err := db.Exec(`
			INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, is_primary, display_order, status, user_id)
			VALUES ($1, 'restricted.jpg', $2, $3, 1024, 'image/jpeg', true, 0, 'CONFIRMED', $4)
		`, toolID, filepath.Join(uploadDir, "restricted.jpg"), filepath.Join(uploadDir, "thumb_restricted.jpg"), ownerID)
		require.NoError(t, err)

		// nonOwnerID shares no org with ownerID and is not the tool's owner.
		ctx, cancel := ContextWithUserIDAndTimeout(nonOwnerID, 5*time.Second)
		defer cancel()

		resp, err := imageClient.GetToolImages(ctx, &pb.GetToolImagesRequest{ToolId: toolID})
		require.NoError(t, err, "as-built, GetToolImages currently has no access restriction to document/lock in")
		assert.Equal(t, 1, len(resp.Images))
	})

	t.Run("SetPrimaryImage", func(t *testing.T) {
		// Setup: Create user, tool, and images
		userID := db.CreateTestUser("e2e-test-setprimary@test.com", "Set Primary User")
		toolID := db.CreateTestTool(userID, "Primary Test Tool", 1000)

		// Get upload directory from config and convert to absolute path
		cfg := loadConfig(t)
		uploadDir := cfg.Storage.UploadDir
		if !filepath.IsAbs(uploadDir) {
			uploadDir = filepath.Join("..", "..", uploadDir)
			uploadDir, _ = filepath.Abs(uploadDir)
		}

		var image1ID, image2ID int32
		err := db.QueryRow(`
			INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, is_primary, status, user_id)
			VALUES ($1, 'primary.jpg', $2, $3, 1024, 'image/jpeg', true, 'CONFIRMED', $4)
			RETURNING id
		`, toolID, filepath.Join(uploadDir, "primary.jpg"), filepath.Join(uploadDir, "thumb_primary.jpg"), userID).Scan(&image1ID)
		require.NoError(t, err)

		err = db.QueryRow(`
			INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, is_primary, status, user_id)
			VALUES ($1, 'secondary.jpg', $2, $3, 1024, 'image/jpeg', false, 'CONFIRMED', $4)
			RETURNING id
		`, toolID, filepath.Join(uploadDir, "secondary.jpg"), filepath.Join(uploadDir, "thumb_secondary.jpg"), userID).Scan(&image2ID)
		require.NoError(t, err)

		// Test: Set image2 as primary
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.SetPrimaryImageRequest{
			ImageId: image2ID,
			ToolId:  toolID,
		}

		resp, err := imageClient.SetPrimaryImage(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Verify: image2 is now primary, image1 is not
		var image1Primary, image2Primary bool
		err = db.QueryRow("SELECT is_primary FROM tool_images WHERE id = $1", image1ID).Scan(&image1Primary)
		assert.NoError(t, err)
		assert.False(t, image1Primary)

		err = db.QueryRow("SELECT is_primary FROM tool_images WHERE id = $1", image2ID).Scan(&image2Primary)
		assert.NoError(t, err)
		assert.True(t, image2Primary)
	})

	t.Run("DeleteImage", func(t *testing.T) {
		// Setup: Create user, tool, and image
		userID := db.CreateTestUser("e2e-test-deleteimage@test.com", "Delete Image User")
		toolID := db.CreateTestTool(userID, "Delete Test Tool", 1000)

		// Get upload directory from config and convert to absolute path
		cfg := loadConfig(t)
		uploadDir := cfg.Storage.UploadDir
		if !filepath.IsAbs(uploadDir) {
			uploadDir = filepath.Join("..", "..", uploadDir)
			uploadDir, _ = filepath.Abs(uploadDir)
		}

		var imageID int32
		err := db.QueryRow(`
			INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, is_primary, status, user_id)
			VALUES ($1, 'todelete.jpg', $2, $3, 1024, 'image/jpeg', false, 'CONFIRMED', $4)
			RETURNING id
		`, toolID, filepath.Join(uploadDir, "todelete.jpg"), filepath.Join(uploadDir, "thumb_todelete.jpg"), userID).Scan(&imageID)
		require.NoError(t, err)

		// Test: Delete image
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.DeleteImageRequest{
			ImageId: imageID,
			ToolId:  toolID,
		}

		resp, err := imageClient.DeleteImage(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Verify: Image is soft deleted (deleted_at is set)
		var deletedAt *time.Time
		err = db.QueryRow("SELECT deleted_at FROM tool_images WHERE id = $1", imageID).Scan(&deletedAt)
		assert.NoError(t, err)
		assert.NotNil(t, deletedAt)
	})

	t.Run("Download Thumbnail", func(t *testing.T) {
		// Setup: Create user, tool, and image with thumbnail
		userID := db.CreateTestUser("e2e-test-thumbnail@test.com", "Thumbnail User")
		toolID := db.CreateTestTool(userID, "Thumbnail Test Tool", 1000)

		// Get upload directory from config and convert to absolute path
		cfg := loadConfig(t)
		uploadDir := cfg.Storage.UploadDir
		if !filepath.IsAbs(uploadDir) {
			uploadDir = filepath.Join("..", "..", uploadDir)
			uploadDir, _ = filepath.Abs(uploadDir)
		}

		// Create a pending image record first to get an ID
		var imageID int32
		storagePath := fmt.Sprintf("tools/%d/temp/withthumb.jpg", toolID)
		err := db.QueryRow(`
			INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, is_primary, status, user_id)
			VALUES ($1, 'withthumb.jpg', $2, $3, 1024, 'image/jpeg', false, 'PENDING', $4)
			RETURNING id
		`, toolID, storagePath, "", userID).Scan(&imageID)
		require.NoError(t, err)

		// Now update with correct paths including the imageID
		filePath := fmt.Sprintf("tools/%d/%d/withthumb.jpg", toolID, imageID)
		thumbnailPath := fmt.Sprintf("tools/%d/%d/thumb_withthumb.jpg", toolID, imageID)
		_, err = db.Exec(`
			UPDATE tool_images 
			SET file_path = $1, thumbnail_path = $2, status = 'CONFIRMED'
			WHERE id = $3
		`, filePath, thumbnailPath, imageID)
		require.NoError(t, err)

		// Create actual thumbnail file on disk
		fullThumbPath := filepath.Join(uploadDir, "images", thumbnailPath)
		os.MkdirAll(filepath.Dir(fullThumbPath), 0755)
		os.WriteFile(fullThumbPath, []byte("dummy thumbnail content"), 0644)

		// Test: Download thumbnail
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 10*time.Second)
		defer cancel()

		req := &pb.GetDownloadUrlRequest{
			ImageId:     imageID,
			ToolId:      toolID,
			IsThumbnail: true,
		}

		resp, err := imageClient.GetDownloadUrl(ctx, req)
		require.NoError(t, err)
		require.NotEmpty(t, resp.DownloadUrl)

		// Download thumbnail
		thumbnailData, err := downloadFileFromURL(resp.DownloadUrl)
		require.NoError(t, err)
		assert.Equal(t, []byte("dummy thumbnail content"), thumbnailData)
	})

	t.Run("Upload Image - Unauthorized User", func(t *testing.T) {
		// Setup: Create two users and a tool owned by user1
		user1ID := db.CreateTestUser("e2e-test-owner-img@test.com", "Owner")
		user2ID := db.CreateTestUser("e2e-test-notowner-img@test.com", "Not Owner")
		toolID := db.CreateTestTool(user1ID, "Owner's Tool", 1000)

		// Test: User2 tries to get upload URL for User1's tool (should fail)
		ctx, cancel := ContextWithUserIDAndTimeout(user2ID, 5*time.Second)
		defer cancel()

		req := &pb.GetUploadUrlRequest{
			ToolId:      toolID,
			Filename:    "unauthorized.jpg",
			ContentType: "image/jpeg",
			IsPrimary:   false,
		}

		_, err := imageClient.GetUploadUrl(ctx, req)
		// Should fail - user2 doesn't own the tool
		assert.Error(t, err)
	})

	t.Run("Confirm Upload - Non-existent Image", func(t *testing.T) {
		// Setup: Create user and tool
		userID := db.CreateTestUser("e2e-test-confirm-fail@test.com", "Confirm Fail User")
		toolID := db.CreateTestTool(userID, "Confirm Fail Tool", 1000)

		// Test: Try to confirm non-existent image
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.ConfirmImageUploadRequest{
			ImageId: 99999, // Non-existent ID
			ToolId:  toolID,
		}

		_, err := imageClient.ConfirmImageUpload(ctx, req)
		assert.Error(t, err)
	})
}

// Helper function to upload file to presigned URL
func uploadFileToURL(url, fileName, mimeType string, data []byte) error {
	// Create HTTP PUT request with raw body (matching S3 presigned URL pattern)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", mimeType)

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// Helper function to download file from presigned URL
func downloadFileFromURL(url string) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}
