package e2e

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageStorageService_E2E(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "")
	defer client.Close()

	imageClient := pb.NewImageStorageServiceClient(client.Conn())

	t.Run("Upload and Download Image", func(t *testing.T) {
		// Setup: Create user and tool
		userID := db.CreateTestUser("e2e-test-image-user@test.com", "Image User")
		orgID := db.CreateTestOrg("")
		db.AddUserToOrg(userID, orgID, "MEMBER", "ACTIVE", 0)
		toolID := db.CreateTestTool(userID, "Image Test Tool", 1000)

		// Create a test image (simple PNG-like data)
		testImageData := []byte{
			0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG header
			// ... simplified for testing
		}

		// Test: Upload image
		ctx1, cancel1 := ContextWithUserIDAndTimeout(userID, 10*time.Second)
		defer cancel1()

		uploadStream, err := imageClient.UploadImage(ctx1)
		require.NoError(t, err)

		// Send metadata
		metadataReq := &pb.UploadImageRequest{
			UploadImageRequestObject: &pb.UploadImageRequestObject{
				ToolId:    toolID,
				FileName:  "test-image.png",
				MimeType:  "image/png",
				IsPrimary: true,
			},
		}
		err = uploadStream.Send(metadataReq)
		require.NoError(t, err)

		// Send image chunks
		chunkSize := 1024
		for i := 0; i < len(testImageData); i += chunkSize {
			end := i + chunkSize
			if end > len(testImageData) {
				end = len(testImageData)
			}

			chunkReq := &pb.UploadImageRequest{
				Chunk: testImageData[i:end],
			}
			err = uploadStream.Send(chunkReq)
			require.NoError(t, err)
		}

		uploadResp, err := uploadStream.CloseAndRecv()
		require.NoError(t, err)
		assert.NotNil(t, uploadResp.ToolImage)
		imageID := uploadResp.ToolImage.Id

		// Verify: Image record in database
		var fileName string
		var isPrimary bool
		err = db.QueryRow("SELECT file_name, is_primary FROM tool_images WHERE id = $1", imageID).Scan(&fileName, &isPrimary)
		assert.NoError(t, err)
		assert.Equal(t, "test-image.png", fileName)
		assert.True(t, isPrimary)

		// Test: Download image
		ctx2, cancel2 := ContextWithUserIDAndTimeout(userID, 10*time.Second)
		defer cancel2()

		downloadReq := &pb.DownloadImageRequest{
			ImageId:     imageID,
			ToolId:      toolID,
			IsThumbnail: false,
		}

		downloadStream, err := imageClient.DownloadImage(ctx2, downloadReq)
		require.NoError(t, err)

		// Receive metadata
		firstResp, err := downloadStream.Recv()
		require.NoError(t, err)
		metadata := firstResp.GetToolImage()
		assert.NotNil(t, metadata)
		assert.Equal(t, "test-image.png", metadata.FileName)

		// Receive chunks
		var downloadedData bytes.Buffer
		for {
			resp, err := downloadStream.Recv()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			chunk := resp.GetChunk()
			if chunk != nil {
				downloadedData.Write(chunk)
			}
		}

		// Verify: Downloaded data matches uploaded data
		assert.Equal(t, testImageData, downloadedData.Bytes())
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
			INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, width, height, is_primary, display_order)
			VALUES 
				($1, 'image1.jpg', $2, $3, 1024, 'image/jpeg', 800, 600, true, 0),
				($1, 'image2.jpg', $4, $5, 2048, 'image/jpeg', 1024, 768, false, 1),
				($1, 'image3.png', $6, $7, 1536, 'image/png', 640, 480, false, 2)
		`, toolID,
			filepath.Join(uploadDir, "image1.jpg"), filepath.Join(uploadDir, "thumb1.jpg"),
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
			INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, width, height, is_primary)
			VALUES ($1, 'primary.jpg', $2, $3, 1024, 'image/jpeg', 800, 600, true)
			RETURNING id
		`, toolID, filepath.Join(uploadDir, "primary.jpg"), filepath.Join(uploadDir, "thumb_primary.jpg")).Scan(&image1ID)
		require.NoError(t, err)

		err = db.QueryRow(`
			INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, width, height, is_primary)
			VALUES ($1, 'secondary.jpg', $2, $3, 1024, 'image/jpeg', 800, 600, false)
			RETURNING id
		`, toolID, filepath.Join(uploadDir, "secondary.jpg"), filepath.Join(uploadDir, "thumb_secondary.jpg")).Scan(&image2ID)
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
			INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, width, height, is_primary)
			VALUES ($1, 'todelete.jpg', $2, $3, 1024, 'image/jpeg', 800, 600, false)
			RETURNING id
		`, toolID, filepath.Join(uploadDir, "todelete.jpg"), filepath.Join(uploadDir, "thumb_todelete.jpg")).Scan(&imageID)
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

		// Verify: Image is soft deleted (deleted_on is set)
		var deletedOn *time.Time
		err = db.QueryRow("SELECT deleted_on FROM tool_images WHERE id = $1", imageID).Scan(&deletedOn)
		assert.NoError(t, err)
		assert.NotNil(t, deletedOn)
	})

	t.Run("Download Thumbnail", func(t *testing.T) {
		// Setup: Create user, tool, and image
		userID := db.CreateTestUser("e2e-test-thumbnail@test.com", "Thumbnail User")
		toolID := db.CreateTestTool(userID, "Thumbnail Test Tool", 1000)

		// Get upload directory from config and convert to absolute path
		cfg := loadConfig(t)
		uploadDir := cfg.Storage.UploadDir
		if !filepath.IsAbs(uploadDir) {
			// Convert relative path to absolute (relative to project root)
			uploadDir = filepath.Join("..", "..", uploadDir)
			uploadDir, _ = filepath.Abs(uploadDir)
		}
		
		// Create dummy file on disk
		os.MkdirAll(uploadDir, 0755)
		thumbPath := filepath.Join(uploadDir, "thumb_withthumb.jpg")
		os.WriteFile(thumbPath, []byte("dummy thumbnail content"), 0644)

		var imageID int32
		err := db.QueryRow(`
			INSERT INTO tool_images (tool_id, file_name, file_path, thumbnail_path, file_size, mime_type, width, height, is_primary)
			VALUES ($1, 'withthumb.jpg', $2, $3, 1024, 'image/jpeg', 800, 600, false)
			RETURNING id
		`, toolID, thumbPath, thumbPath).Scan(&imageID)
		require.NoError(t, err)

		// Test: Download thumbnail
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 10*time.Second)
		defer cancel()

		req := &pb.DownloadImageRequest{
			ImageId:     imageID,
			ToolId:      toolID,
			IsThumbnail: true, // Request thumbnail
		}

		stream, err := imageClient.DownloadImage(ctx, req)
		require.NoError(t, err)

		// Receive metadata
		firstResp, err := stream.Recv()
		require.NoError(t, err)
		metadata := firstResp.GetToolImage()
		assert.NotNil(t, metadata)
		// Thumbnail should be indicated in the response
	})

	t.Run("Upload Image - Unauthorized User", func(t *testing.T) {
		// Setup: Create two users and a tool owned by user1
		user1ID := db.CreateTestUser("e2e-test-owner-img@test.com", "Owner")
		user2ID := db.CreateTestUser("e2e-test-notowner-img@test.com", "Not Owner")
		toolID := db.CreateTestTool(user1ID, "Owner's Tool", 1000)

		// Test: User2 tries to upload image to User1's tool (should fail)
		ctx, cancel := ContextWithUserIDAndTimeout(user2ID, 5*time.Second)
		defer cancel()

		stream, err := imageClient.UploadImage(ctx)
		require.NoError(t, err)

		metadataReq := &pb.UploadImageRequest{
			UploadImageRequestObject: &pb.UploadImageRequestObject{
				ToolId:    toolID,
				FileName:  "unauthorized.jpg",
				MimeType:  "image/jpeg",
				IsPrimary: false,
			},
		}
		err = stream.Send(metadataReq)
		require.NoError(t, err)

		_, err = stream.CloseAndRecv()
		// Should fail - user2 doesn't own the tool
		assert.Error(t, err)
	})
}

