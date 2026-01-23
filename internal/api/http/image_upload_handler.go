package http

import (
	"io"
	"net/http"
	"path/filepath"

	"ubertool-backend-trusted/internal/storage"

	"github.com/gorilla/mux"
)

// ImageUploadHandler handles HTTP uploads for mock storage
type ImageUploadHandler struct {
	mockStorage *storage.MockStorageService
}

// NewImageUploadHandler creates a new upload handler
func NewImageUploadHandler(mockStorage *storage.MockStorageService) *ImageUploadHandler {
	return &ImageUploadHandler{
		mockStorage: mockStorage,
	}
}

// HandleMockUpload handles HTTP PUT requests to mock presigned URLs
func (h *ImageUploadHandler) HandleMockUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get storage key from query parameter
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing key parameter", http.StatusBadRequest)
		return
	}

	// Validate content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/gif" {
		http.Error(w, "Invalid content type", http.StatusBadRequest)
		return
	}

	// Save file
	err := h.mockStorage.SaveFile(key, r.Body)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Return success (mimic S3 response)
	w.Header().Set("ETag", `"mock-etag-success"`)
	w.WriteHeader(http.StatusOK)
}

// HandleMockDownload handles HTTP GET requests to download images
func (h *ImageUploadHandler) HandleMockDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get storage key from query parameter
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing key parameter", http.StatusBadRequest)
		return
	}

	// Read file
	file, err := h.mockStorage.ReadFile(key)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// Determine content type from file extension
	ext := filepath.Ext(key)
	contentType := "application/octet-stream"
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	}

	// Set headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// Stream file
	io.Copy(w, file)
}

// RegisterMockStorageRoutes registers the mock storage HTTP endpoints
func RegisterMockStorageRoutes(router *mux.Router, mockStorage *storage.MockStorageService) {
	handler := NewImageUploadHandler(mockStorage)
	router.HandleFunc("/api/v1/upload/{token}", handler.HandleMockUpload).Methods("PUT")
	router.HandleFunc("/api/v1/download/{key}", handler.HandleMockDownload).Methods("GET")
}
