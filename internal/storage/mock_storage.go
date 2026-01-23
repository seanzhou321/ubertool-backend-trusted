package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// MockStorageService implements image storage using local filesystem
// This is for demo/testing without AWS S3 or Azure Blob Storage
type MockStorageService struct {
	baseURL      string // Server URL (e.g., "http://localhost:8080")
	uploadsDir   string // Local directory for uploads (e.g., "./uploads")
	imagesDir    string // Subdirectory for images
	thumbnailDir string // Subdirectory for thumbnails
}

// NewMockStorageService creates a new mock storage service
func NewMockStorageService(baseURL, uploadsDir string) (*MockStorageService, error) {
	imagesDir := filepath.Join(uploadsDir, "images")
	thumbnailDir := filepath.Join(uploadsDir, "thumbnails")

	// Create directories if they don't exist
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create images directory: %w", err)
	}
	if err := os.MkdirAll(thumbnailDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create thumbnails directory: %w", err)
	}

	return &MockStorageService{
		baseURL:      baseURL,
		uploadsDir:   uploadsDir,
		imagesDir:    imagesDir,
		thumbnailDir: thumbnailDir,
	}, nil
}

// GeneratePresignedUploadURL generates a mock upload URL pointing to the server
func (m *MockStorageService) GeneratePresignedUploadURL(
	ctx context.Context,
	key string,
	contentType string,
	expiresIn time.Duration,
) (string, error) {
	// Generate unique upload token
	uploadToken := uuid.New().String()

	// Create mock presigned URL pointing to server
	// The key is encoded in the query parameter so the upload handler knows where to save
	uploadURL := fmt.Sprintf("%s/api/v1/upload/%s?key=%s", m.baseURL, uploadToken, key)

	return uploadURL, nil
}

// GeneratePresignedDownloadURL generates a mock download URL
func (m *MockStorageService) GeneratePresignedDownloadURL(
	ctx context.Context,
	key string,
	expiresIn time.Duration,
) (string, error) {
	// Encode the key for URL safety
	encodedKey := encodeKey(key)

	// Generate download URL pointing to server
	// The actual key is in query parameter
	downloadURL := fmt.Sprintf("%s/api/v1/download/%s?key=%s", m.baseURL, encodedKey, key)

	return downloadURL, nil
}

// FileExists checks if file exists in local filesystem
func (m *MockStorageService) FileExists(ctx context.Context, key string) (bool, int64, error) {
	fullPath := filepath.Join(m.imagesDir, key)

	// Debug logging
	fmt.Printf("[DEBUG FileExists] key=%s, fullPath=%s\n", key, fullPath)

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("[DEBUG FileExists] File does not exist: %s\n", fullPath)
			return false, 0, nil
		}
		fmt.Printf("[DEBUG FileExists] Stat error: %v\n", err)
		return false, 0, err
	}

	fmt.Printf("[DEBUG FileExists] File found, size=%d\n", info.Size())
	return true, info.Size(), nil
}

// DeleteFile deletes file from local filesystem
func (m *MockStorageService) DeleteFile(ctx context.Context, key string) error {
	fullPath := filepath.Join(m.imagesDir, key)

	err := os.Remove(fullPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// SaveFile saves uploaded file to local filesystem
func (m *MockStorageService) SaveFile(key string, reader io.Reader) error {
	// Determine full path
	fullPath := filepath.Join(m.imagesDir, key)

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data
	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ReadFile reads file from local filesystem
func (m *MockStorageService) ReadFile(key string) (io.ReadCloser, error) {
	fullPath := filepath.Join(m.imagesDir, key)

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// GetLocalPath returns the filesystem path for a key
func (m *MockStorageService) GetLocalPath(key string) string {
	return filepath.Join(m.imagesDir, key)
}

// encodeKey creates a URL-safe hash of the key
func encodeKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes
}
