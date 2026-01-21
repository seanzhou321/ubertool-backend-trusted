package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type imageStorageService struct {
	toolRepo   repository.ToolRepository
	uploadPath string
}

func NewImageStorageService(toolRepo repository.ToolRepository, uploadPath string) ImageStorageService {
	// Ensure upload directory exists
	if _, err := os.Stat(uploadPath); os.IsNotExist(err) {
		_ = os.MkdirAll(uploadPath, 0755)
	}
	return &imageStorageService{
		toolRepo:   toolRepo,
		uploadPath: uploadPath,
	}
}

func (s *imageStorageService) UploadImage(ctx context.Context, userID, toolID int32, file []byte, filename, mimeType string) (*domain.ToolImage, error) {
	// 0. Authorization check: Does user own the tool?
	tool, err := s.toolRepo.GetByID(ctx, toolID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify tool ownership: %w", err)
	}
	if tool.OwnerID != userID {
		return nil, fmt.Errorf("unauthorized: you do not own this tool")
	}

	// 1. Save file to disk
	// Generate unique filename to avoid collision
	uniqueName := fmt.Sprintf("%d_%d_%s", toolID, time.Now().UnixNano(), filename)
	filePath := filepath.Join(s.uploadPath, uniqueName)

	if err := os.WriteFile(filePath, file, 0644); err != nil {
		return nil, fmt.Errorf("failed to save image file: %w", err)
	}

	// 2. Check if first image
	existing, _ := s.toolRepo.GetImages(ctx, toolID)
	isPrimary := len(existing) == 0

	// 3. Create ToolImage record
	// Note: We don't have real thumbnail logic or dimension extraction here without external libraries.
	// For now, we'll placeholder them or use 0/empty.
	img := &domain.ToolImage{
		ToolID:        toolID,
		FileName:      filename,
		FilePath:      filePath,
		ThumbnailPath: filePath, // Use same for now
		FileSize:      int32(len(file)),
		MimeType:      mimeType,
		Width:         0, // Placeholder
		Height:        0, // Placeholder
		IsPrimary:     isPrimary,
		DisplayOrder:  int32(len(existing)),
	}

	if err := s.toolRepo.AddImage(ctx, img); err != nil {
		return nil, fmt.Errorf("failed to save image metadata: %w", err)
	}

	return img, nil
}

func (s *imageStorageService) GetToolImages(ctx context.Context, toolID int32) ([]domain.ToolImage, error) {
	return s.toolRepo.GetImages(ctx, toolID)
}

func (s *imageStorageService) DownloadImage(ctx context.Context, toolID, imageID int32, isThumbnail bool) ([]byte, string, error) {
	images, err := s.toolRepo.GetImages(ctx, toolID)
	if err != nil {
		return nil, "", err
	}

	var targetImg *domain.ToolImage
	for _, img := range images {
		if img.ID == imageID {
			targetImg = &img
			break
		}
	}
	if targetImg == nil {
		return nil, "", fmt.Errorf("image not found")
	}

	path := targetImg.FilePath
	if isThumbnail && targetImg.ThumbnailPath != "" {
		path = targetImg.ThumbnailPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	return data, targetImg.MimeType, nil
}

func (s *imageStorageService) DeleteImage(ctx context.Context, imageID int32) error {
	// Also delete from disk? We need path.
	// Without GetImageByID, we can't look up path to delete file.
	// Soft delete in DB is fine for now as per schema `deleted_on`.
	return s.toolRepo.DeleteImage(ctx, imageID)
}

func (s *imageStorageService) SetPrimaryImage(ctx context.Context, toolID, imageID int32) error {
	return s.toolRepo.SetPrimaryImage(ctx, toolID, imageID)
}
