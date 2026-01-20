# Local File Storage Design for Tool Sharing App MVP

## Overview

This document outlines the design and implementation of local file storage for tool images in a neighborhood tool sharing application. This approach is suitable for MVP/demo stages before migrating to cloud storage.

## Architecture Design

### Directory Structure

```
/var/app/uploads/
├── tools/
│   ├── {tool-id-1}/
│   │   ├── original_image1.jpg
│   │   ├── original_image2.jpg
│   │   └── thumbnails/
│   │       ├── thumb_image1.jpg
│   │       └── thumb_image2.jpg
│   ├── {tool-id-2}/
│   │   ├── original_image1.jpg
│   │   └── thumbnails/
│   │       └── thumb_image1.jpg
│   └── temp/
│       └── {upload-session-id}/
```

### Key Design Principles

1. **Isolation by Tool**: Each tool gets its own directory for easy management and cleanup
2. **Thumbnail Generation**: Automatic thumbnail creation for list views and performance
3. **Atomic Uploads**: Use temporary directory during upload, move on completion
4. **File Naming**: Use UUID-based names to avoid conflicts and prevent path traversal attacks

## Database Schema

```sql
-- Tools table (assumed existing)
CREATE TABLE tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    owner_id UUID NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Tool images table
CREATE TABLE tool_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_id UUID NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
    filename VARCHAR(255) NOT NULL,
    original_filename VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    thumbnail_path VARCHAR(500),
    file_size INTEGER NOT NULL,
    mime_type VARCHAR(50) NOT NULL,
    width INTEGER,
    height INTEGER,
    is_primary BOOLEAN DEFAULT FALSE,
    display_order INTEGER DEFAULT 0,
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_primary_per_tool UNIQUE(tool_id, is_primary) WHERE is_primary = TRUE
);

-- Index for fast queries
CREATE INDEX idx_tool_images_tool_id ON tool_images(tool_id);
CREATE INDEX idx_tool_images_primary ON tool_images(tool_id, is_primary);
```

## gRPC API Methods

All gRPC methods use Protocol Buffers for serialization and support streaming for large files.

### 1. Upload Tool Image

**Method**: `rpc UploadImage(stream UploadImageRequest) returns (UploadImageResponse)`

**Request Stream**:
First message contains metadata:
```protobuf
message UploadImageRequest {
  ImageMetadata metadata = 1;
}

message ImageMetadata {
  string tool_id = 1;
  string user_id = 2;
  string original_filename = 3;
  string mime_type = 4;
  bool is_primary = 5;
}
```

Subsequent messages contain file chunks:
```protobuf
message UploadImageRequest {
  bytes chunk = 2;
}
```

**Response**:
```protobuf
message UploadImageResponse {
  string image_id = 1;
  string tool_id = 2;
  string filename = 3;
  string url = 4;
  string thumbnail_url = 5;
  int64 file_size = 6;
  bool is_primary = 7;
  google.protobuf.Timestamp uploaded_at = 8;
}
```

**Usage Example**:
```go
// Client sends metadata first
stream.Send(&UploadImageRequest{
  Data: &UploadImageRequest_Metadata{
    Metadata: &ImageMetadata{
      ToolId: "tool-123",
      UserId: "user-456",
      OriginalFilename: "drill.jpg",
      MimeType: "image/jpeg",
      IsPrimary: true,
    },
  },
})

// Then send file in chunks
for {
  n, _ := file.Read(buffer)
  stream.Send(&UploadImageRequest{
    Data: &UploadImageRequest_Chunk{Chunk: buffer[:n]},
  })
}

response := stream.CloseAndRecv()
```

### 2. Get Tool Images

**Method**: `rpc GetToolImages(GetToolImagesRequest) returns (GetToolImagesResponse)`

**Request**:
```protobuf
message GetToolImagesRequest {
  string tool_id = 1;
}
```

**Response**:
```protobuf
message GetToolImagesResponse {
  repeated ToolImage images = 1;
}

message ToolImage {
  string id = 1;
  string tool_id = 2;
  string filename = 3;
  string url = 4;
  string thumbnail_url = 5;
  int64 file_size = 6;
  int32 width = 7;
  int32 height = 8;
  bool is_primary = 9;
  int32 display_order = 10;
  google.protobuf.Timestamp uploaded_at = 11;
}
```

**Usage Example**:
```go
response, err := client.GetToolImages(ctx, &GetToolImagesRequest{
  ToolId: "tool-123",
})
// response.Images contains all images for the tool
```

### 3. Download Image File

**Method**: `rpc DownloadImage(DownloadImageRequest) returns (stream DownloadImageResponse)`

**Request**:
```protobuf
message DownloadImageRequest {
  string tool_id = 1;
  string image_id = 2;
  bool thumbnail = 3; // true for thumbnail, false for original
}
```

**Response Stream**:
First message contains file info:
```protobuf
message DownloadImageResponse {
  ImageInfo info = 1;
}

message ImageInfo {
  string filename = 1;
  string mime_type = 2;
  int64 file_size = 3;
}
```

Subsequent messages contain file chunks:
```protobuf
message DownloadImageResponse {
  bytes chunk = 2;
}
```

**Usage Example**:
```go
stream, err := client.DownloadImage(ctx, &DownloadImageRequest{
  ToolId: "tool-123",
  ImageId: "image-789",
  Thumbnail: false,
})

// Receive file info
resp, _ := stream.Recv()
info := resp.GetInfo()

// Receive chunks
for {
  resp, err := stream.Recv()
  if err == io.EOF { break }
  chunk := resp.GetChunk()
  file.Write(chunk)
}
```

### 4. Delete Tool Image

**Method**: `rpc DeleteImage(DeleteImageRequest) returns (DeleteImageResponse)`

**Request**:
```protobuf
message DeleteImageRequest {
  string tool_id = 1;
  string image_id = 2;
  string user_id = 3;
}
```

**Response**:
```protobuf
message DeleteImageResponse {
  bool success = 1;
}
```

**Usage Example**:
```go
response, err := client.DeleteImage(ctx, &DeleteImageRequest{
  ToolId: "tool-123",
  ImageId: "image-789",
  UserId: "user-456",
})
// response.Success indicates if deletion was successful
```

### 5. Set Primary Image

**Method**: `rpc SetPrimaryImage(SetPrimaryImageRequest) returns (SetPrimaryImageResponse)`

**Request**:
```protobuf
message SetPrimaryImageRequest {
  string tool_id = 1;
  string image_id = 2;
  string user_id = 3;
}
```

**Response**:
```protobuf
message SetPrimaryImageResponse {
  bool success = 1;
}
```

**Usage Example**:
```go
response, err := client.SetPrimaryImage(ctx, &SetPrimaryImageRequest{
  ToolId: "tool-123",
  ImageId: "image-789",
  UserId: "user-456",
})
// response.Success indicates if operation was successful
```

## gRPC Error Codes

The service uses standard gRPC status codes for error handling:

| Error Scenario | gRPC Status Code | Description |
|---------------|------------------|-------------|
| Invalid file type | `INVALID_ARGUMENT` | Unsupported MIME type |
| File too large | `INVALID_ARGUMENT` | Exceeds 10MB limit |
| Tool not found | `NOT_FOUND` | Tool ID doesn't exist |
| Access denied | `PERMISSION_DENIED` | User doesn't own the tool |
| Image not found | `NOT_FOUND` | Image ID doesn't exist |
| Server error | `INTERNAL` | Unexpected server error |

**Error Handling Example**:
```go
response, err := client.UploadImage(stream)
if err != nil {
  if st, ok := status.FromError(err); ok {
    switch st.Code() {
    case codes.InvalidArgument:
      fmt.Println("Invalid file:", st.Message())
    case codes.PermissionDenied:
      fmt.Println("Access denied:", st.Message())
    default:
      fmt.Println("Error:", st.Message())
    }
  }
}
```

## Implementation Examples

### gRPC Protocol Buffers Definition

First, define your protobuf service:

```protobuf
// proto/image_service.proto
syntax = "proto3";

package toolsharing.image.v1;

option go_package = "github.com/yourorg/toolsharing/gen/go/image/v1;imagev1";

import "google/protobuf/timestamp.proto";

// ImageService handles tool image uploads and retrieval
service ImageService {
  // Upload a tool image using streaming for large files
  rpc UploadImage(stream UploadImageRequest) returns (UploadImageResponse);
  
  // Get image metadata for a tool
  rpc GetToolImages(GetToolImagesRequest) returns (GetToolImagesResponse);
  
  // Download image using streaming
  rpc DownloadImage(DownloadImageRequest) returns (stream DownloadImageResponse);
  
  // Delete a tool image
  rpc DeleteImage(DeleteImageRequest) returns (DeleteImageResponse);
  
  // Set primary image for a tool
  rpc SetPrimaryImage(SetPrimaryImageRequest) returns (SetPrimaryImageResponse);
}

// Upload request - streaming chunks
message UploadImageRequest {
  oneof data {
    ImageMetadata metadata = 1;
    bytes chunk = 2;
  }
}

message ImageMetadata {
  string tool_id = 1;
  string user_id = 2;
  string original_filename = 3;
  string mime_type = 4;
  bool is_primary = 5;
}

message UploadImageResponse {
  string image_id = 1;
  string tool_id = 2;
  string filename = 3;
  string url = 4;
  string thumbnail_url = 5;
  int64 file_size = 6;
  bool is_primary = 7;
  google.protobuf.Timestamp uploaded_at = 8;
}

message GetToolImagesRequest {
  string tool_id = 1;
}

message GetToolImagesResponse {
  repeated ToolImage images = 1;
}

message ToolImage {
  string id = 1;
  string tool_id = 2;
  string filename = 3;
  string url = 4;
  string thumbnail_url = 5;
  int64 file_size = 6;
  int32 width = 7;
  int32 height = 8;
  bool is_primary = 9;
  int32 display_order = 10;
  google.protobuf.Timestamp uploaded_at = 11;
}

message DownloadImageRequest {
  string tool_id = 1;
  string image_id = 2;
  bool thumbnail = 3; // true for thumbnail, false for original
}

message DownloadImageResponse {
  oneof data {
    ImageInfo info = 1;
    bytes chunk = 2;
  }
}

message ImageInfo {
  string filename = 1;
  string mime_type = 2;
  int64 file_size = 3;
}

message DeleteImageRequest {
  string tool_id = 1;
  string image_id = 2;
  string user_id = 3;
}

message DeleteImageResponse {
  bool success = 1;
}

message SetPrimaryImageRequest {
  string tool_id = 1;
  string image_id = 2;
  string user_id = 3;
}

message SetPrimaryImageResponse {
  bool success = 1;
}
```

### Go Implementation - Image Service

```go
// internal/service/image_service.go
package service

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	imagev1 "github.com/yourorg/toolsharing/gen/go/image/v1"
	"github.com/yourorg/toolsharing/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	uploadDir        = "/var/app/uploads/tools"
	maxFileSize      = 10 * 1024 * 1024 // 10MB
	thumbnailSize    = 300
	chunkSize        = 64 * 1024 // 64KB chunks
)

var allowedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

type ImageServiceServer struct {
	imagev1.UnimplementedImageServiceServer
	repo repository.ImageRepository
}

func NewImageServiceServer(repo repository.ImageRepository) *ImageServiceServer {
	return &ImageServiceServer{
		repo: repo,
	}
}

// UploadImage handles streaming image upload
func (s *ImageServiceServer) UploadImage(stream imagev1.ImageService_UploadImageServer) error {
	var metadata *imagev1.ImageMetadata
	var tempFile *os.File
	var bytesReceived int64
	
	// Create temp directory
	tempDir := filepath.Join(uploadDir, "temp", uuid.New().String())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return status.Errorf(codes.Internal, "failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Receive streamed data
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive chunk: %v", err)
		}

		switch data := req.Data.(type) {
		case *imagev1.UploadImageRequest_Metadata:
			// First message contains metadata
			metadata = data.Metadata
			
			// Validate metadata
			if err := s.validateMetadata(metadata); err != nil {
				return err
			}
			
			// Verify tool ownership
			if err := s.repo.VerifyToolOwnership(stream.Context(), metadata.ToolId, metadata.UserId); err != nil {
				return status.Errorf(codes.PermissionDenied, "tool not found or access denied")
			}
			
			// Create temp file
			ext := s.getFileExtension(metadata.OriginalFilename)
			tempPath := filepath.Join(tempDir, fmt.Sprintf("upload%s", ext))
			tempFile, err = os.Create(tempPath)
			if err != nil {
				return status.Errorf(codes.Internal, "failed to create temp file: %v", err)
			}

		case *imagev1.UploadImageRequest_Chunk:
			// Subsequent messages contain file chunks
			if tempFile == nil {
				return status.Errorf(codes.InvalidArgument, "metadata must be sent before chunks")
			}
			
			// Check file size limit
			bytesReceived += int64(len(data.Chunk))
			if bytesReceived > maxFileSize {
				tempFile.Close()
				return status.Errorf(codes.InvalidArgument, "file size exceeds maximum of %d bytes", maxFileSize)
			}
			
			// Write chunk to temp file
			if _, err := tempFile.Write(data.Chunk); err != nil {
				tempFile.Close()
				return status.Errorf(codes.Internal, "failed to write chunk: %v", err)
			}
		}
	}

	if tempFile == nil {
		return status.Errorf(codes.InvalidArgument, "no file data received")
	}
	tempFile.Close()

	// Process the uploaded file
	imageID := uuid.New().String()
	result, err := s.processImage(stream.Context(), tempFile.Name(), metadata, imageID, bytesReceived)
	if err != nil {
		return err
	}

	return stream.SendAndClose(result)
}

func (s *ImageServiceServer) processImage(
	ctx context.Context,
	tempPath string,
	metadata *imagev1.ImageMetadata,
	imageID string,
	fileSize int64,
) (*imagev1.UploadImageResponse, error) {
	
	// Open and validate image
	imgFile, err := os.Open(tempPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to open temp file: %v", err)
	}
	defer imgFile.Close()

	// Decode image to get dimensions
	img, format, err := image.DecodeConfig(imgFile)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid image file: %v", err)
	}
	
	// Verify mime type matches actual format
	expectedMime := fmt.Sprintf("image/%s", format)
	if metadata.MimeType != expectedMime && metadata.MimeType != "image/jpg" {
		return nil, status.Errorf(codes.InvalidArgument, "mime type mismatch")
	}

	// Create tool-specific directories
	toolDir := filepath.Join(uploadDir, metadata.ToolId)
	thumbnailDir := filepath.Join(toolDir, "thumbnails")
	if err := os.MkdirAll(thumbnailDir, 0755); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create directories: %v", err)
	}

	// Generate unique filename
	ext := s.getFileExtension(metadata.OriginalFilename)
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	finalPath := filepath.Join(toolDir, filename)
	thumbnailPath := filepath.Join(thumbnailDir, fmt.Sprintf("thumb_%s", filename))

	// Move file to final location
	if err := os.Rename(tempPath, finalPath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to move file: %v", err)
	}

	// Generate thumbnail
	if err := s.generateThumbnail(finalPath, thumbnailPath); err != nil {
		os.Remove(finalPath) // Cleanup on error
		return nil, status.Errorf(codes.Internal, "failed to generate thumbnail: %v", err)
	}

	// Save to database
	imageRecord := &repository.ToolImage{
		ID:               imageID,
		ToolID:           metadata.ToolId,
		Filename:         filename,
		OriginalFilename: metadata.OriginalFilename,
		FilePath:         finalPath,
		ThumbnailPath:    thumbnailPath,
		FileSize:         fileSize,
		MimeType:         metadata.MimeType,
		Width:            int32(img.Width),
		Height:           int32(img.Height),
		IsPrimary:        metadata.IsPrimary,
		UploadedAt:       time.Now(),
	}

	if err := s.repo.CreateImage(ctx, imageRecord); err != nil {
		os.Remove(finalPath)
		os.Remove(thumbnailPath)
		return nil, status.Errorf(codes.Internal, "failed to save image metadata: %v", err)
	}

	return &imagev1.UploadImageResponse{
		ImageId:      imageID,
		ToolId:       metadata.ToolId,
		Filename:     filename,
		Url:          fmt.Sprintf("/api/images/tools/%s/%s", metadata.ToolId, filename),
		ThumbnailUrl: fmt.Sprintf("/api/images/tools/%s/thumbnails/thumb_%s", metadata.ToolId, filename),
		FileSize:     fileSize,
		IsPrimary:    metadata.IsPrimary,
		UploadedAt:   timestamppb.New(imageRecord.UploadedAt),
	}, nil
}

// GetToolImages retrieves all images for a tool
func (s *ImageServiceServer) GetToolImages(
	ctx context.Context,
	req *imagev1.GetToolImagesRequest,
) (*imagev1.GetToolImagesResponse, error) {
	
	images, err := s.repo.GetToolImages(ctx, req.ToolId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get images: %v", err)
	}

	response := &imagev1.GetToolImagesResponse{
		Images: make([]*imagev1.ToolImage, len(images)),
	}

	for i, img := range images {
		response.Images[i] = &imagev1.ToolImage{
			Id:            img.ID,
			ToolId:        img.ToolID,
			Filename:      img.Filename,
			Url:           fmt.Sprintf("/api/images/tools/%s/%s", img.ToolID, img.Filename),
			ThumbnailUrl:  fmt.Sprintf("/api/images/tools/%s/thumbnails/thumb_%s", img.ToolID, img.Filename),
			FileSize:      img.FileSize,
			Width:         img.Width,
			Height:        img.Height,
			IsPrimary:     img.IsPrimary,
			DisplayOrder:  img.DisplayOrder,
			UploadedAt:    timestamppb.New(img.UploadedAt),
		}
	}

	return response, nil
}

// DownloadImage streams an image file to the client
func (s *ImageServiceServer) DownloadImage(
	req *imagev1.DownloadImageRequest,
	stream imagev1.ImageService_DownloadImageServer,
) error {
	
	// Get image metadata from database
	img, err := s.repo.GetImage(stream.Context(), req.ImageId)
	if err != nil {
		return status.Errorf(codes.NotFound, "image not found")
	}

	// Verify tool ID matches
	if img.ToolID != req.ToolId {
		return status.Errorf(codes.InvalidArgument, "tool ID mismatch")
	}

	// Determine which file to send
	filePath := img.FilePath
	if req.Thumbnail {
		filePath = img.ThumbnailPath
	}

	// Validate file path to prevent path traversal
	if strings.Contains(filePath, "..") {
		return status.Errorf(codes.InvalidArgument, "invalid file path")
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return status.Errorf(codes.NotFound, "file not found: %v", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to get file info: %v", err)
	}

	// Send file info first
	if err := stream.Send(&imagev1.DownloadImageResponse{
		Data: &imagev1.DownloadImageResponse_Info{
			Info: &imagev1.ImageInfo{
				Filename: img.Filename,
				MimeType: img.MimeType,
				FileSize: fileInfo.Size(),
			},
		},
	}); err != nil {
		return status.Errorf(codes.Internal, "failed to send file info: %v", err)
	}

	// Stream file chunks
	buffer := make([]byte, chunkSize)
	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to read file: %v", err)
		}

		if err := stream.Send(&imagev1.DownloadImageResponse{
			Data: &imagev1.DownloadImageResponse_Chunk{
				Chunk: buffer[:n],
			},
		}); err != nil {
			return status.Errorf(codes.Internal, "failed to send chunk: %v", err)
		}
	}

	return nil
}

// DeleteImage deletes an image file and its metadata
func (s *ImageServiceServer) DeleteImage(
	ctx context.Context,
	req *imagev1.DeleteImageRequest,
) (*imagev1.DeleteImageResponse, error) {
	
	// Verify tool ownership
	if err := s.repo.VerifyToolOwnership(ctx, req.ToolId, req.UserId); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "tool not found or access denied")
	}

	// Get image metadata
	img, err := s.repo.GetImage(ctx, req.ImageId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "image not found")
	}

	// Verify tool ID matches
	if img.ToolID != req.ToolId {
		return nil, status.Errorf(codes.InvalidArgument, "tool ID mismatch")
	}

	// Delete files
	os.Remove(img.FilePath)
	os.Remove(img.ThumbnailPath)

	// Delete from database
	if err := s.repo.DeleteImage(ctx, req.ImageId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete image: %v", err)
	}

	return &imagev1.DeleteImageResponse{Success: true}, nil
}

// SetPrimaryImage sets an image as the primary image for a tool
func (s *ImageServiceServer) SetPrimaryImage(
	ctx context.Context,
	req *imagev1.SetPrimaryImageRequest,
) (*imagev1.SetPrimaryImageResponse, error) {
	
	// Verify tool ownership
	if err := s.repo.VerifyToolOwnership(ctx, req.ToolId, req.UserId); err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "tool not found or access denied")
	}

	if err := s.repo.SetPrimaryImage(ctx, req.ToolId, req.ImageId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set primary image: %v", err)
	}

	return &imagev1.SetPrimaryImageResponse{Success: true}, nil
}

// Helper functions

func (s *ImageServiceServer) validateMetadata(metadata *imagev1.ImageMetadata) error {
	if metadata.ToolId == "" {
		return status.Errorf(codes.InvalidArgument, "tool_id is required")
	}
	if metadata.UserId == "" {
		return status.Errorf(codes.InvalidArgument, "user_id is required")
	}
	if !allowedMimeTypes[metadata.MimeType] {
		return status.Errorf(codes.InvalidArgument, "unsupported mime type: %s", metadata.MimeType)
	}
	return nil
}

func (s *ImageServiceServer) getFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return ".jpg"
	}
	return strings.ToLower(ext)
}

func (s *ImageServiceServer) generateThumbnail(srcPath, dstPath string) error {
	// Open source image
	src, err := imaging.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}

	// Create thumbnail
	thumb := imaging.Fill(src, thumbnailSize, thumbnailSize, imaging.Center, imaging.Lanczos)

	// Save thumbnail
	if err := imaging.Save(thumb, dstPath); err != nil {
		return fmt.Errorf("failed to save thumbnail: %w", err)
	}

	return nil
}
```

### Go Implementation - Repository Layer

```go
// internal/repository/image_repository.go
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
)

type ToolImage struct {
	ID               string
	ToolID           string
	Filename         string
	OriginalFilename string
	FilePath         string
	ThumbnailPath    string
	FileSize         int64
	MimeType         string
	Width            int32
	Height           int32
	IsPrimary        bool
	DisplayOrder     int32
	UploadedAt       time.Time
}

type ImageRepository interface {
	CreateImage(ctx context.Context, img *ToolImage) error
	GetImage(ctx context.Context, imageID string) (*ToolImage, error)
	GetToolImages

(ctx context.Context, toolID string) ([]*ToolImage, error)
	DeleteImage(ctx context.Context, imageID string) error
	SetPrimaryImage(ctx context.Context, toolID, imageID string) error
	VerifyToolOwnership(ctx context.Context, toolID, userID string) error
}

type PostgresImageRepository struct {
	db *sql.DB
}

func NewPostgresImageRepository(db *sql.DB) *PostgresImageRepository {
	return &PostgresImageRepository{db: db}
}

func (r *PostgresImageRepository) CreateImage(ctx context.Context, img *ToolImage) error {
	query := `
		INSERT INTO tool_images (
			id, tool_id, filename, original_filename, file_path, thumbnail_path,
			file_size, mime_type, width, height, is_primary, uploaded_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.db.ExecContext(ctx, query,
		img.ID, img.ToolID, img.Filename, img.OriginalFilename,
		img.FilePath, img.ThumbnailPath, img.FileSize, img.MimeType,
		img.Width, img.Height, img.IsPrimary, img.UploadedAt,
	)
	return err
}

func (r *PostgresImageRepository) GetImage(ctx context.Context, imageID string) (*ToolImage, error) {
	query := `
		SELECT id, tool_id, filename, original_filename, file_path, thumbnail_path,
			   file_size, mime_type, width, height, is_primary, display_order, uploaded_at
		FROM tool_images
		WHERE id = $1
	`
	img := &ToolImage{}
	err := r.db.QueryRowContext(ctx, query, imageID).Scan(
		&img.ID, &img.ToolID, &img.Filename, &img.OriginalFilename,
		&img.FilePath, &img.ThumbnailPath, &img.FileSize, &img.MimeType,
		&img.Width, &img.Height, &img.IsPrimary, &img.DisplayOrder, &img.UploadedAt,
	)
	if err == sql.ErrNoRows {
		return nil, err
	}
	return img, err
}

func (r *PostgresImageRepository) GetToolImages(ctx context.Context, toolID string) ([]*ToolImage, error) {
	query := `
		SELECT id, tool_id, filename, original_filename, file_path, thumbnail_path,
			   file_size, mime_type, width, height, is_primary, display_order, uploaded_at
		FROM tool_images
		WHERE tool_id = $1
		ORDER BY is_primary DESC, display_order ASC, uploaded_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, toolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []*ToolImage
	for rows.Next() {
		img := &ToolImage{}
		if err := rows.Scan(
			&img.ID, &img.ToolID, &img.Filename, &img.OriginalFilename,
			&img.FilePath, &img.ThumbnailPath, &img.FileSize, &img.MimeType,
			&img.Width, &img.Height, &img.IsPrimary, &img.DisplayOrder, &img.UploadedAt,
		); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

func (r *PostgresImageRepository) DeleteImage(ctx context.Context, imageID string) error {
	query := `DELETE FROM tool_images WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, imageID)
	return err
}

func (r *PostgresImageRepository) SetPrimaryImage(ctx context.Context, toolID, imageID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unset all primary images for this tool
	_, err = tx.ExecContext(ctx, `UPDATE tool_images SET is_primary = false WHERE tool_id = $1`, toolID)
	if err != nil {
		return err
	}

	// Set the new primary image
	_, err = tx.ExecContext(ctx, `UPDATE tool_images SET is_primary = true WHERE id = $1 AND tool_id = $2`, imageID, toolID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PostgresImageRepository) VerifyToolOwnership(ctx context.Context, toolID, userID string) error {
	query := `SELECT 1 FROM tools WHERE id = $1 AND owner_id = $2`
	var exists int
	err := r.db.QueryRowContext(ctx, query, toolID, userID).Scan(&exists)
	return err
}
```

### Server Setup - Main Entry Point

```go
// cmd/image-service/main.go
package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
	imagev1 "github.com/yourorg/toolsharing/gen/go/image/v1"
	"github.com/yourorg/toolsharing/internal/repository"
	"github.com/yourorg/toolsharing/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Database connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@localhost:5432/toolsharing?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Database connection established")

	// Initialize repository
	imageRepo := repository.NewPostgresImageRepository(db)

	// Initialize gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(10*1024*1024), // 10MB max message size
		grpc.MaxSendMsgSize(10*1024*1024),
	)

	// Register image service
	imageService := service.NewImageServiceServer(imageRepo)
	imagev1.RegisterImageServiceServer(grpcServer, imageService)

	// Enable reflection for grpcurl/grpcui
	reflection.Register(grpcServer)

	// Start server
	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50051"
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("gRPC server listening on :%s", port)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down gRPC server...")
		grpcServer.GracefulStop()
	}()

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
```

### Mobile Client Example (Go/gRPC)

```go
// client/image_client.go
package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	imagev1 "github.com/yourorg/toolsharing/gen/go/image/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ImageClient struct {
	client imagev1.ImageServiceClient
}

func NewImageClient(serverAddr string) (*ImageClient, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &ImageClient{
		client: imagev1.NewImageServiceClient(conn),
	}, nil
}

// UploadImage uploads an image file to the server
func (c *ImageClient) UploadImage(ctx context.Context, toolID, userID, filePath string, isPrimary bool) (*imagev1.UploadImageResponse, error) {
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Determine mime type based on extension
	ext := filepath.Ext(filePath)
	mimeType := "image/jpeg"
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".webp":
		mimeType = "image/webp"
	}

	// Create streaming client
	stream, err := c.client.UploadImage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	// Send metadata first
	metadata := &imagev1.ImageMetadata{
		ToolId:           toolID,
		UserId:           userID,
		OriginalFilename: filepath.Base(filePath),
		MimeType:         mimeType,
		IsPrimary:        isPrimary,
	}

	if err := stream.Send(&imagev1.UploadImageRequest{
		Data: &imagev1.UploadImageRequest_Metadata{
			Metadata: metadata,
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to send metadata: %w", err)
	}

	// Stream file chunks
	buffer := make([]byte, 64*1024) // 64KB chunks
	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		if err := stream.Send(&imagev1.UploadImageRequest{
			Data: &imagev1.UploadImageRequest_Chunk{
				Chunk: buffer[:n],
			},
		}); err != nil {
			return nil, fmt.Errorf("failed to send chunk: %w", err)
		}
	}

	// Close and receive response
	response, err := stream.CloseAndRecv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	fmt.Printf("Uploaded image: %s (size: %d bytes)\n", response.Filename, fileInfo.Size())
	return response, nil
}

// GetToolImages retrieves all images for a tool
func (c *ImageClient) GetToolImages(ctx context.Context, toolID string) ([]*imagev1.ToolImage, error) {
	response, err := c.client.GetToolImages(ctx, &imagev1.GetToolImagesRequest{
		ToolId: toolID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get images: %w", err)
	}

	return response.Images, nil
}

// DownloadImage downloads an image to a local file
func (c *ImageClient) DownloadImage(ctx context.Context, toolID, imageID, outputPath string, thumbnail bool) error {
	stream, err := c.client.DownloadImage(ctx, &imagev1.DownloadImageRequest{
		ToolId:    toolID,
		ImageId:   imageID,
		Thumbnail: thumbnail,
	})
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Receive first message (metadata)
	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive metadata: %w", err)
	}

	info := resp.GetInfo()
	if info == nil {
		return fmt.Errorf("expected metadata in first message")
	}

	fmt.Printf("Downloading: %s (%d bytes)\n", info.Filename, info.FileSize)

	// Receive and write chunks
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive chunk: %w", err)
		}

		chunk := resp.GetChunk()
		if chunk != nil {
			if _, err := outFile.Write(chunk); err != nil {
				return fmt.Errorf("failed to write chunk: %w", err)
			}
		}
	}

	fmt.Println("Download complete")
	return nil
}

// DeleteImage deletes an image
func (c *ImageClient) DeleteImage(ctx context.Context, toolID, imageID, userID string) error {
	response, err := c.client.DeleteImage(ctx, &imagev1.DeleteImageRequest{
		ToolId:  toolID,
		ImageId: imageID,
		UserId:  userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete image: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("delete operation failed")
	}

	fmt.Println("Image deleted successfully")
	return nil
}
```

### Example Usage

```go
// example/main.go
package main

import (
	"context"
	"log"

	"github.com/yourorg/toolsharing/client"
)

func main() {
	// Connect to gRPC server
	imageClient, err := client.NewImageClient("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// Upload an image
	response, err := imageClient.UploadImage(
		ctx,
		"tool-123",              // toolID
		"user-456",              // userID
		"/path/to/image.jpg",    // file path
		true,                    // is primary
	)
	if err != nil {
		log.Fatalf("Upload failed: %v", err)
	}
	log.Printf("Uploaded: %s", response.ImageId)

	// Get all images for a tool
	images, err := imageClient.GetToolImages(ctx, "tool-123")
	if err != nil {
		log.Fatalf("Failed to get images: %v", err)
	}
	log.Printf("Found %d images", len(images))

	// Download an image
	err = imageClient.DownloadImage(
		ctx,
		"tool-123",
		response.ImageId,
		"/path/to/download.jpg",
		false, // download original, not thumbnail
	)
	if err != nil {
		log.Fatalf("Download failed: %v", err)
	}

	// Delete an image
	err = imageClient.DeleteImage(ctx, "tool-123", response.ImageId, "user-456")
	if err != nil {
		log.Fatalf("Delete failed: %v", err)
	}
}
```

### Dockerfile

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /image-service ./cmd/image-service

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary
COPY --from=builder /image-service .

# Create upload directory
RUN mkdir -p /var/app/uploads/tools

# Expose port
EXPOSE 50051

CMD ["./image-service"]
```

### Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: toolsharing
      POSTGRES_USER: user
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql

  image-service:
    build: .
    ports:
      - "50051:50051"
    environment:
      DATABASE_URL: postgres://user:password@postgres:5432/toolsharing?sslmode=disable
      GRPC_PORT: 50051
    volumes:
      - image_uploads:/var/app/uploads/tools
    depends_on:
      - postgres

volumes:
  postgres_data:
  image_uploads:
```

## Security Considerations

### 1. File Validation
- **MIME Type Check**: Verify file type on server, don't trust client
- **Magic Bytes**: Check file headers to prevent disguised malicious files
- **File Size Limit**: Enforce maximum file size (10MB recommended)

```javascript
// Enhanced validation
const fileTypeFromBuffer = require('file-type');

async function validateImageFile(buffer) {
  const type = await fileTypeFromBuffer(buffer);
  
  if (!type || !ALLOWED_MIME_TYPES.includes(type.mime)) {
    throw new Error('Invalid file type');
  }
  
  return type;
}
```

### 2. Path Traversal Prevention
- Never use user-provided filenames directly
- Use UUID-based naming
- Validate all path components
- Use `path.join()` safely

### 3. Access Control
- Verify tool ownership before upload
- Check permissions before serving images
- Implement rate limiting on uploads

### 4. File Permissions
```bash
# Set proper permissions on upload directory
chmod 755 /var/app/uploads/tools
chown app-user:app-group /var/app/uploads/tools
```

## Backup Strategy

### Daily Backup Script

```bash
#!/bin/bash
# backup-images.sh

UPLOAD_DIR="/var/app/uploads/tools"
BACKUP_DIR="/var/backups/tool-images"
DATE=$(date +%Y%m%d)

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Create compressed backup
tar -czf "$BACKUP_DIR/tools-images-$DATE.tar.gz" -C "$UPLOAD_DIR" .

# Keep only last 7 days
find "$BACKUP_DIR" -name "tools-images-*.tar.gz" -mtime +7 -delete

# Verify backup
if [ -f "$BACKUP_DIR/tools-images-$DATE.tar.gz" ]; then
    echo "Backup successful: tools-images-$DATE.tar.gz"
else
    echo "Backup failed!" >&2
    exit 1
fi
```

### Cron Job Setup
```bash
# Run daily at 2 AM
0 2 * * * /usr/local/bin/backup-images.sh >> /var/log/image-backup.log 2>&1
```

## Performance Optimization

### 1. Image Optimization on Upload

```javascript
// Compress and optimize images
await sharp(filePath)
  .jpeg({ quality: 85, progressive: true })
  .withMetadata()
  .toFile(optimizedPath);
```

### 2. Caching Headers

```javascript
// Serve images with caching
router.get('/images/tools/:toolId/:filename', (req, res) => {
  res.setHeader('Cache-Control', 'public, max-age=31536000'); // 1 year
  res.setHeader('ETag', generateETag(filePath));
  res.sendFile(filePath);
});
```

### 3. Lazy Loading on Mobile

```javascript
// Mobile app - React Native example
<Image
  source={{ uri: imageUrl }}
  resizeMode="cover"
  defaultSource={require('./placeholder.png')}
/>
```

## Cleanup Tasks

### Orphaned File Cleanup

```javascript
// cleanup-orphaned-files.js
async function cleanupOrphanedFiles() {
  const toolDirs = await fs.readdir(UPLOAD_DIR);
  
  for (const toolId of toolDirs) {
    if (toolId === 'temp') continue;
    
    // Check if tool exists in database
    const tool = await db.query('SELECT id FROM tools WHERE id = $1', [toolId]);
    
    if (tool.rows.length === 0) {
      // Tool deleted, remove directory
      await fs.rm(path.join(UPLOAD_DIR, toolId), { recursive: true });
      console.log(`Removed orphaned directory: ${toolId}`);
    }
  }
}

// Run weekly
```

## Migration Path to Cloud Storage

When ready to migrate to cloud storage (S3, etc.):

1. **Dual Write**: Write to both local and cloud during transition
2. **Background Migration**: Script to upload existing files
3. **Update URLs**: Migrate database URLs to cloud URLs
4. **Verify**: Ensure all images accessible
5. **Cleanup**: Remove local files after verification

```javascript
// Migration script example
async function migrateToS3() {
  const images = await db.query('SELECT * FROM tool_images');
  
  for (const image of images.rows) {
    // Upload to S3
    const s3Url = await uploadToS3(image.file_path);
    
    // Update database
    await db.query(
      'UPDATE tool_images SET cloud_url = $1 WHERE id = $2',
      [s3Url, image.id]
    );
  }
}
```

## Monitoring and Maintenance

### Metrics to Track
- Total storage used
- Number of images per tool
- Upload success/failure rate
- Average image size
- Disk space remaining

### Log Examples
```javascript
logger.info('Image uploaded', {
  toolId,
  imageId,
  fileSize,
  mimeType,
  userId,
  timestamp: new Date().toISOString()
});
```

## Testing Checklist

- [ ] Upload valid image (JPEG, PNG, WebP)
- [ ] Upload invalid file type
- [ ] Upload oversized file (>10MB)
- [ ] Upload with malicious filename
- [ ] Retrieve image successfully
- [ ] Retrieve non-existent image
- [ ] Delete image as owner
- [ ] Delete image as non-owner
- [ ] Thumbnail generation
- [ ] Primary image setting
- [ ] Multiple images per tool
- [ ] Concurrent uploads

## Conclusion

This local file storage implementation provides a solid foundation for your MVP. It's simple, reliable, and easy to migrate to cloud storage later. The key benefits are:

- No external dependencies or costs
- Full control over storage
- Simple backup procedures
- Easy local development

When you're ready to scale, the migration path to cloud storage is straightforward.