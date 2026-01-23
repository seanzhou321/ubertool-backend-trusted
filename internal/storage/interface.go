package storage

import (
	"context"
	"io"
	"time"
)

// StorageInterface defines the interface for image storage backends
// Supports both mock (local filesystem) and cloud storage (S3, Azure, etc.)
type StorageInterface interface {
	// GeneratePresignedUploadURL generates a presigned URL for uploading
	// key: storage path/key for the file
	// contentType: MIME type (e.g., "image/jpeg")
	// expiresIn: how long the URL should be valid
	GeneratePresignedUploadURL(ctx context.Context, key string, contentType string, expiresIn time.Duration) (string, error)

	// GeneratePresignedDownloadURL generates a presigned URL for downloading
	// key: storage path/key for the file
	// expiresIn: how long the URL should be valid
	GeneratePresignedDownloadURL(ctx context.Context, key string, expiresIn time.Duration) (string, error)

	// FileExists checks if a file exists and returns its size
	FileExists(ctx context.Context, key string) (exists bool, size int64, err error)

	// DeleteFile removes a file from storage
	DeleteFile(ctx context.Context, key string) error

	// SaveFile saves a file (used by mock storage HTTP handler)
	// This is only needed for mock implementation
	SaveFile(key string, reader io.Reader) error

	// ReadFile opens a file for reading (used by mock storage HTTP handler)
	// This is only needed for mock implementation
	ReadFile(key string) (io.ReadCloser, error)
}
