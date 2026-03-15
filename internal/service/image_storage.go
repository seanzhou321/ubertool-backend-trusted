package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"path"
	"time"

	xdraw "golang.org/x/image/draw"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/logger"
	"ubertool-backend-trusted/internal/repository"
	"ubertool-backend-trusted/internal/storage"
)

type imageStorageService struct {
	toolRepo repository.ToolRepository
	userRepo repository.UserRepository
	orgRepo  repository.OrganizationRepository
	storage  storage.StorageInterface
}

func NewImageStorageService(
	toolRepo repository.ToolRepository,
	userRepo repository.UserRepository,
	orgRepo repository.OrganizationRepository,
	storage storage.StorageInterface,
) ImageStorageService {
	return &imageStorageService{
		toolRepo: toolRepo,
		userRepo: userRepo,
		orgRepo:  orgRepo,
		storage:  storage,
	}
}

// GetUploadUrl generates a presigned URL for uploading an image
func (s *imageStorageService) GetUploadUrl(
	ctx context.Context,
	userID int32,
	filename, contentType string,
	toolID int32,
	isPrimary bool,
) (*domain.ToolImage, string, string, int64, error) {
	// Verify tool ownership
	tool, err := s.toolRepo.GetByID(ctx, toolID)
	if err != nil {
		return nil, "", "", 0, fmt.Errorf("failed to verify tool: %w", err)
	}
	if tool.OwnerID != userID {
		return nil, "", "", 0, fmt.Errorf("unauthorized: you do not own this tool")
	}

	// Determine storage path
	storagePath := fmt.Sprintf("tools/%d/%s", toolID, filename)

	// Create pending image record (ID will be auto-generated)
	expiresAt := time.Now().Add(15 * time.Minute)

	image := &domain.ToolImage{
		ToolID:        toolID,
		UserID:        userID,
		FileName:      filename,
		FilePath:      storagePath,
		ThumbnailPath: "",
		FileSize:      0,
		MimeType:      contentType,
		IsPrimary:     isPrimary,
		DisplayOrder:  0,
		Status:        "PENDING",
		ExpiresAt:     &expiresAt,
		CreatedOn:     time.Now(),
	}

	// Create image record - database will generate ID
	if err := s.toolRepo.CreateImage(ctx, image); err != nil {
		return nil, "", "", 0, fmt.Errorf("failed to create image record: %w", err)
	}

	// Now we have the generated ID, update storage path to include it
	storagePath = fmt.Sprintf("tools/%d/%d/%s", toolID, image.ID, filename)

	// Update the image record with the correct path including ID
	image.FilePath = storagePath
	if err := s.toolRepo.UpdateImage(ctx, image); err != nil {
		return nil, "", "", 0, fmt.Errorf("failed to update image path: %w", err)
	}

	// Generate presigned upload URL (15 minutes expiration)
	uploadURL, err := s.storage.GeneratePresignedUploadURL(ctx, storagePath, contentType, 15*time.Minute)
	if err != nil {
		return nil, "", "", 0, fmt.Errorf("failed to generate upload URL: %w", err)
	}

	// Generate presigned download URL (1 hour expiration)
	downloadURL, err := s.storage.GeneratePresignedDownloadURL(ctx, storagePath, 1*time.Hour)
	if err != nil {
		return nil, "", "", 0, fmt.Errorf("failed to generate download URL: %w", err)
	}

	return image, uploadURL, downloadURL, expiresAt.Unix(), nil
}

// ConfirmImageUpload confirms that an image was successfully uploaded
func (s *imageStorageService) ConfirmImageUpload(
	ctx context.Context,
	userID int32,
	imageID int32,
	toolID int32,
	fileSize int64,
) (*domain.ToolImage, error) {
	// Get pending image
	image, err := s.toolRepo.GetImageByID(ctx, imageID)
	if err != nil {
		return nil, fmt.Errorf("image not found: %w", err)
	}

	// Verify ownership
	if image.UserID != userID {
		return nil, fmt.Errorf("unauthorized: you do not own this image")
	}

	// Verify image is pending
	if image.Status != "PENDING" {
		return nil, fmt.Errorf("image is not pending (status: %s)", image.Status)
	}

	// Check if file exists in storage
	exists, actualSize, err := s.storage.FileExists(ctx, image.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to verify file: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("image file not found in storage")
	}

	// Update image record
	if fileSize == 0 {
		fileSize = actualSize
	}
	image.FileSize = fileSize
	image.Status = "CONFIRMED"
	now := time.Now()
	image.ConfirmedOn = &now

	// If this is the first image for the tool, set as primary
	existingImages, err := s.toolRepo.GetImages(ctx, toolID)
	if err == nil && len(existingImages) == 0 {
		image.IsPrimary = true
	}

	if err := s.toolRepo.UpdateImage(ctx, image); err != nil {
		return nil, fmt.Errorf("failed to update image: %w", err)
	}

	// Kick off thumbnail generation asynchronously so the RPC returns immediately.
	go s.generateThumbnail(image.ID, image.FilePath)

	return image, nil
}

// generateThumbnail reads the confirmed image from storage, resizes it to fit within
// 300×300 pixels (preserving aspect ratio), saves the result as JPEG, and updates
// the thumbnail_path column. Runs in a background goroutine.
func (s *imageStorageService) generateThumbnail(imageID int32, filePath string) {
	ctx := context.Background()

	// Derive thumbnail storage key beside the original, always as JPEG.
	dir := path.Dir(filePath)
	base := path.Base(filePath)
	ext := path.Ext(base)
	stem := base[:len(base)-len(ext)]
	thumbnailPath := path.Join(dir, "thumb_"+stem+".jpg")

	// Read original image from storage.
	reader, err := s.storage.ReadFile(filePath)
	if err != nil {
		logger.Error("thumbnail: failed to read image", "image_id", imageID, "error", err)
		return
	}
	defer reader.Close()

	// Decode (JPEG and PNG are registered; add more blank imports above for other formats).
	src, _, err := image.Decode(reader)
	if err != nil {
		logger.Error("thumbnail: failed to decode image", "image_id", imageID, "error", err)
		return
	}

	// Resize to fit within 300×300 preserving aspect ratio.
	dst := resizeToFit(src, 300, 300)

	// Encode as JPEG.
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		logger.Error("thumbnail: failed to encode", "image_id", imageID, "error", err)
		return
	}

	// Persist thumbnail.
	if err := s.storage.SaveFile(thumbnailPath, &buf); err != nil {
		logger.Error("thumbnail: failed to save", "image_id", imageID, "error", err)
		return
	}

	// Update the DB record with the thumbnail path.
	img, err := s.toolRepo.GetImageByID(ctx, imageID)
	if err != nil {
		logger.Error("thumbnail: failed to fetch image record", "image_id", imageID, "error", err)
		return
	}
	img.ThumbnailPath = thumbnailPath
	if err := s.toolRepo.UpdateImage(ctx, img); err != nil {
		logger.Error("thumbnail: failed to update record", "image_id", imageID, "error", err)
		return
	}

	logger.Info("thumbnail: generated", "image_id", imageID, "path", thumbnailPath)
}

// resizeToFit scales src so that it fits within maxW×maxH while preserving the
// original aspect ratio. If the source already fits, it is returned unchanged.
func resizeToFit(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	srcW, srcH := b.Dx(), b.Dy()

	// Already small enough — no scaling needed.
	if srcW <= maxW && srcH <= maxH {
		return src
	}

	scaleW := float64(maxW) / float64(srcW)
	scaleH := float64(maxH) / float64(srcH)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}

	dstW := int(float64(srcW) * scale)
	dstH := int(float64(srcH) * scale)
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

// GetDownloadUrl generates a presigned URL for downloading an image
func (s *imageStorageService) GetDownloadUrl(
	ctx context.Context,
	userID int32,
	imageID int32,
	toolID int32,
	isThumbnail bool,
) (string, int64, error) {
	// Get images for the tool
	images, err := s.toolRepo.GetImages(ctx, toolID)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get tool images: %w", err)
	}

	// Find the specific image
	var targetImage *domain.ToolImage
	for i := range images {
		if images[i].ID == imageID {
			targetImage = &images[i]
			break
		}
	}

	if targetImage == nil {
		return "", 0, fmt.Errorf("image not found")
	}

	// Verify access to tool (basic check - could be enhanced with org membership)
	tool, err := s.toolRepo.GetByID(ctx, toolID)
	if err != nil {
		return "", 0, fmt.Errorf("failed to verify tool: %w", err)
	}

	// Allow access if user owns the tool or tool is available (public)
	if tool.OwnerID != userID && tool.Status != domain.ToolStatusAvailable {
		return "", 0, fmt.Errorf("unauthorized: no access to this tool's images")
	}

	// Determine which path to use
	path := targetImage.FilePath
	if isThumbnail && targetImage.ThumbnailPath != "" {
		path = targetImage.ThumbnailPath
	}

	// Generate presigned download URL (1 hour expiration)
	downloadURL, err := s.storage.GeneratePresignedDownloadURL(ctx, path, 1*time.Hour)
	if err != nil {
		return "", 0, fmt.Errorf("failed to generate download URL: %w", err)
	}

	expiresAt := time.Now().Add(1 * time.Hour).Unix()
	return downloadURL, expiresAt, nil
}

// GetToolImages retrieves all confirmed images for a tool
func (s *imageStorageService) GetToolImages(ctx context.Context, toolID int32) ([]domain.ToolImage, error) {
	return s.toolRepo.GetImages(ctx, toolID)
}

// DeleteImage deletes an image and its files from storage
func (s *imageStorageService) DeleteImage(
	ctx context.Context,
	userID int32,
	imageID int32,
	toolID int32,
) error {
	// Get image
	image, err := s.toolRepo.GetImageByID(ctx, imageID)
	if err != nil {
		return fmt.Errorf("image not found: %w", err)
	}

	// Verify ownership through tool
	tool, err := s.toolRepo.GetByID(ctx, toolID)
	if err != nil {
		return fmt.Errorf("failed to verify tool: %w", err)
	}
	if tool.OwnerID != userID {
		return fmt.Errorf("unauthorized: you do not own this tool")
	}

	// Delete files from storage
	if err := s.storage.DeleteFile(ctx, image.FilePath); err != nil {
		// Log error but continue - file might already be deleted
		fmt.Printf("Warning: failed to delete image file: %v\n", err)
	}
	if image.ThumbnailPath != "" && image.ThumbnailPath != image.FilePath {
		if err := s.storage.DeleteFile(ctx, image.ThumbnailPath); err != nil {
			fmt.Printf("Warning: failed to delete thumbnail: %v\n", err)
		}
	}

	// Soft delete in database
	if err := s.toolRepo.DeleteImage(ctx, imageID); err != nil {
		return fmt.Errorf("failed to delete image record: %w", err)
	}

	// If this was the primary image, set another as primary
	if image.IsPrimary {
		images, err := s.toolRepo.GetImages(ctx, toolID)
		if err == nil && len(images) > 0 {
			// Set the first remaining image as primary
			s.toolRepo.SetPrimaryImage(ctx, toolID, images[0].ID)
		}
	}

	return nil
}

// SetPrimaryImage sets a specific image as the primary image for a tool
func (s *imageStorageService) SetPrimaryImage(
	ctx context.Context,
	userID int32,
	toolID int32,
	imageID int32,
) error {
	// Verify tool ownership
	tool, err := s.toolRepo.GetByID(ctx, toolID)
	if err != nil {
		return fmt.Errorf("failed to verify tool: %w", err)
	}
	if tool.OwnerID != userID {
		return fmt.Errorf("unauthorized: you do not own this tool")
	}

	// Verify image exists and belongs to this tool
	image, err := s.toolRepo.GetImageByID(ctx, imageID)
	if err != nil {
		return fmt.Errorf("image not found: %w", err)
	}
	if image.ToolID != toolID {
		return fmt.Errorf("image does not belong to this tool")
	}
	if image.Status != "CONFIRMED" {
		return fmt.Errorf("image is not confirmed")
	}

	// Set as primary
	return s.toolRepo.SetPrimaryImage(ctx, toolID, imageID)
}
