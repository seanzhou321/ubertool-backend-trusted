package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

	// uploadTokens/downloadTokens back the presigned-URL contract for the local HTTP endpoints
	// registered by internal/api/http/image_upload_handler.go, which sit outside the gRPC auth
	// interceptor chain: a token minted here is the *only* access control those endpoints have
	// (see sbr/rtm/009-security.rtm.md SEC-HTTP-001). Each token authorizes exactly one key,
	// mirroring real S3 presigned URLs, which stay valid for repeated use throughout their
	// expiry window rather than being single-use.
	uploadTokens   sync.Map // map[string]tokenRecord
	downloadTokens sync.Map // map[string]tokenRecord
}

// tokenRecord binds an issued presigned-URL token to the one storage key it authorizes and the
// time it stops being valid.
type tokenRecord struct {
	key       string
	expiresAt time.Time
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

	m := &MockStorageService{
		baseURL:      baseURL,
		uploadsDir:   uploadsDir,
		imagesDir:    imagesDir,
		thumbnailDir: thumbnailDir,
	}
	go m.cleanupExpiredTokensLoop()
	return m, nil
}

// cleanupExpiredTokensLoop evicts expired upload/download tokens so the token maps don't grow
// unbounded over the process lifetime. Mirrors internal/security/rate_limiter.go's cleanup
// pattern.
func (m *MockStorageService) cleanupExpiredTokensLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		m.uploadTokens.Range(func(k, v any) bool {
			if now.After(v.(tokenRecord).expiresAt) {
				m.uploadTokens.Delete(k)
			}
			return true
		})
		m.downloadTokens.Range(func(k, v any) bool {
			if now.After(v.(tokenRecord).expiresAt) {
				m.downloadTokens.Delete(k)
			}
			return true
		})
	}
}

// GeneratePresignedUploadURL generates a mock upload URL pointing to the server
func (m *MockStorageService) GeneratePresignedUploadURL(
	ctx context.Context,
	key string,
	contentType string,
	expiresIn time.Duration,
) (string, error) {
	uploadToken := uuid.New().String()
	m.uploadTokens.Store(uploadToken, tokenRecord{key: key, expiresAt: time.Now().Add(expiresIn)})

	// Create mock presigned URL pointing to server
	// The key is encoded in the query parameter so the upload handler knows where to save
	uploadURL := fmt.Sprintf("%s/api/v1/upload/%s?key=%s", m.baseURL, uploadToken, key)

	return uploadURL, nil
}

// ValidateUploadToken reports whether token was issued by GeneratePresignedUploadURL for
// exactly this key and has not yet expired.
func (m *MockStorageService) ValidateUploadToken(token, key string) bool {
	v, ok := m.uploadTokens.Load(token)
	if !ok {
		return false
	}
	rec := v.(tokenRecord)
	return rec.key == key && time.Now().Before(rec.expiresAt)
}

// GeneratePresignedDownloadURL generates a mock download URL
func (m *MockStorageService) GeneratePresignedDownloadURL(
	ctx context.Context,
	key string,
	expiresIn time.Duration,
) (string, error) {
	downloadToken := uuid.New().String()
	m.downloadTokens.Store(downloadToken, tokenRecord{key: key, expiresAt: time.Now().Add(expiresIn)})

	// Generate download URL pointing to server
	// The actual key is in query parameter
	downloadURL := fmt.Sprintf("%s/api/v1/download/%s?key=%s", m.baseURL, downloadToken, key)

	return downloadURL, nil
}

// ValidateDownloadToken reports whether token was issued by GeneratePresignedDownloadURL for
// exactly this key and has not yet expired.
func (m *MockStorageService) ValidateDownloadToken(token, key string) bool {
	v, ok := m.downloadTokens.Load(token)
	if !ok {
		return false
	}
	rec := v.(tokenRecord)
	return rec.key == key && time.Now().Before(rec.expiresAt)
}

// safeJoin resolves key against baseDir and rejects any result that would escape baseDir (e.g.
// key="../../etc/passwd") — see sbr/rtm/009-security.rtm.md SEC-HTTP-002. filepath.Join alone
// does not provide this guarantee: it happily normalizes ".." segments right out of the base
// directory.
func safeJoin(baseDir, key string) (string, error) {
	baseDir = filepath.Clean(baseDir)
	full := filepath.Clean(filepath.Join(baseDir, key))
	if full != baseDir && !strings.HasPrefix(full, baseDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid key %q: resolves outside the storage directory", key)
	}
	return full, nil
}

// FileExists checks if file exists in local filesystem
func (m *MockStorageService) FileExists(ctx context.Context, key string) (bool, int64, error) {
	fullPath, err := safeJoin(m.imagesDir, key)
	if err != nil {
		return false, 0, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, err
	}

	return true, info.Size(), nil
}

// DeleteFile deletes file from local filesystem
func (m *MockStorageService) DeleteFile(ctx context.Context, key string) error {
	fullPath, err := safeJoin(m.imagesDir, key)
	if err != nil {
		return err
	}

	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// SaveFile saves uploaded file to local filesystem
func (m *MockStorageService) SaveFile(key string, reader io.Reader) error {
	fullPath, err := safeJoin(m.imagesDir, key)
	if err != nil {
		return err
	}

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
	fullPath, err := safeJoin(m.imagesDir, key)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// GetLocalPath returns the filesystem path for a key
func (m *MockStorageService) GetLocalPath(key string) string {
	fullPath, err := safeJoin(m.imagesDir, key)
	if err != nil {
		return ""
	}
	return fullPath
}
