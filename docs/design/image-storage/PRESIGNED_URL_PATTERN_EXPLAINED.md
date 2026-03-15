# Presigned URL Pattern for Image Upload/Download

**Date:** January 22, 2026  
**Author:** System Architecture Team  
**Status:** Recommended Implementation

---

## Executive Summary

The presigned URL pattern is a **cloud-native approach** for handling file uploads and downloads that **offloads data transfer from your application server to cloud storage** (AWS S3, Google Cloud Storage, Azure Blob Storage, etc.).

### Key Concept

Instead of:
```
Client → Your Server → Cloud Storage (streaming through your server)
```

You do:
```
Client → Your Server (just get URL)
Client → Cloud Storage (direct upload/download)
Your Server → Verifies upload completed
```

---

## Why Replace Streaming with Presigned URLs?

### Current Problem: gRPC Streaming

Your current proto defines streaming image upload/download:

```protobuf
// Current (Streaming)
rpc UploadImage(stream UploadImageRequest) returns (UploadImageResponse);
rpc DownloadImage(DownloadImageRequest) returns (stream DownloadImageResponse);
```

**Issues with Streaming:**
1. **Server Bandwidth:** Every image byte goes through your server
2. **Server Load:** CPU and memory used for streaming
3. **Complexity:** Requires async/streaming implementation (complex code)
4. **Scalability:** Server becomes bottleneck as users grow
5. **Cost:** High bandwidth costs on server

### Solution: Presigned URL Pattern

```protobuf
// New (Presigned URLs)
rpc GetUploadUrl(GetUploadUrlRequest) returns (GetUploadUrlResponse);
rpc ConfirmImageUpload(ConfirmImageUploadRequest) returns (ConfirmImageUploadResponse);
rpc GetDownloadUrl(GetDownloadUrlRequest) returns (GetDownloadUrlResponse);
```

**Benefits:**
1. ✅ **Zero server bandwidth** for image data
2. ✅ **Minimal server load** (just generate URLs)
3. ✅ **Simple code** (standard HTTP PUT/GET)
4. ✅ **Infinite scalability** (cloud storage handles traffic)
5. ✅ **Lower costs** (pay-as-you-go storage, no bandwidth)

---

## How Presigned URLs Work

### Concept: Temporary Permission Tokens

A presigned URL is a **time-limited URL with embedded authentication** that allows anyone with the URL to perform a specific operation (upload or download) without needing credentials.

**Analogy:** It's like a temporary guest pass to a building:
- Valid for specific action (upload or download)
- Valid for specific resource (specific file)
- Expires after a set time (15 minutes for upload, 1 hour for download)
- No other credentials needed

### Technical Process

```
┌─────────────────────────────────────────────────────────┐
│                  Upload Flow                            │
└─────────────────────────────────────────────────────────┘

1. Client: "I want to upload tool_123.jpg"
   ↓
2. Your Server:
   - Generates UUID: "abc-123-def"
   - Creates storage path: "images/org_5/tool_123/abc-123-def/tool_123.jpg"
   - Asks S3: "Give me presigned PUT URL for this path, expires in 15 min"
   - S3 returns: "https://s3.amazonaws.com/bucket/path?credentials=..."
   - Server creates pending record in database
   - Returns to client: {upload_url, image_id, download_url, expires_at}
   ↓
3. Client:
   - Uses standard HTTP PUT to upload_url
   - Sends image bytes directly to S3
   - S3 stores the file
   - No interaction with your server during upload!
   ↓
4. Client: "I finished uploading image_id=abc-123-def"
   ↓
5. Your Server:
   - Checks S3: "Does file exist at path?"
   - S3: "Yes, 2.3 MB file exists"
   - Server updates database: status='CONFIRMED', file_size=2.3MB
   - Schedules thumbnail generation (async job)
   - Returns: {success=true, tool_image object}
```

### Download Flow

```
┌─────────────────────────────────────────────────────────┐
│                  Download Flow                          │
└─────────────────────────────────────────────────────────┘

1. Client: "I want to display image_id=abc-123-def"
   ↓
2. Your Server:
   - Verifies permissions (user can see this tool?)
   - Asks S3: "Give me presigned GET URL, expires in 1 hour"
   - S3 returns: "https://s3.amazonaws.com/bucket/path?credentials=..."
   - Returns to client: {download_url, expires_at}
   ↓
3. Client:
   - Uses standard HTTP GET to download_url
   - Downloads image bytes directly from S3
   - Displays image
   - No interaction with your server during download!
```

---

## Complete Implementation Guide

### Step 1: Set Up Cloud Storage

#### Option A: AWS S3 (Recommended)

1. Create S3 bucket: `ubertool-images-prod`
2. Configure bucket policy:
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Principal": {"AWS": "arn:aws:iam::YOUR_ACCOUNT:role/ubertool-backend"},
         "Action": [
           "s3:PutObject",
           "s3:GetObject",
           "s3:DeleteObject"
         ],
         "Resource": "arn:aws:s3:::ubertool-images-prod/*"
       }
     ]
   }
   ```

3. Enable CORS:
   ```json
   [
     {
       "AllowedOrigins": ["https://your-app.com"],
       "AllowedMethods": ["PUT", "GET"],
       "AllowedHeaders": ["*"],
       "ExposeHeaders": ["ETag"],
       "MaxAgeSeconds": 3000
     }
   ]
   ```

4. Configure lifecycle rules (optional):
   - Delete pending uploads after 24 hours
   - Move to Glacier after 90 days (for old images)

#### Option B: Google Cloud Storage

Similar setup with GCS bucket and service account permissions.

---

### Step 2: Backend Implementation (Go)

#### 2.1: Install Dependencies

```go
// go.mod
require (
    github.com/aws/aws-sdk-go-v2 v1.17.0
    github.com/aws/aws-sdk-go-v2/config v1.18.0
    github.com/aws/aws-sdk-go-v2/service/s3 v1.30.0
    github.com/google/uuid v1.3.0
)
```

#### 2.2: Create Storage Service

```go
// internal/storage/s3_service.go
package storage

import (
    "context"
    "fmt"
    "time"
    
    "github.com/aws/aws-sdk-go-v2/aws"
    v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/google/uuid"
)

type S3Service struct {
    client     *s3.Client
    bucketName string
    region     string
    presigner  *s3.PresignClient
}

func NewS3Service(cfg aws.Config, bucketName string) *S3Service {
    client := s3.NewFromConfig(cfg)
    return &S3Service{
        client:     client,
        bucketName: bucketName,
        region:     cfg.Region,
        presigner:  s3.NewPresignClient(client),
    }
}

// GeneratePresignedUploadURL generates a presigned PUT URL
func (s *S3Service) GeneratePresignedUploadURL(
    ctx context.Context,
    key string,
    contentType string,
    expiresIn time.Duration,
) (string, error) {
    req, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
        Bucket:      aws.String(s.bucketName),
        Key:         aws.String(key),
        ContentType: aws.String(contentType),
    }, func(opts *s3.PresignOptions) {
        opts.Expires = expiresIn
    })
    
    if err != nil {
        return "", fmt.Errorf("failed to generate presigned URL: %w", err)
    }
    
    return req.URL, nil
}

// GeneratePresignedDownloadURL generates a presigned GET URL
func (s *S3Service) GeneratePresignedDownloadURL(
    ctx context.Context,
    key string,
    expiresIn time.Duration,
) (string, error) {
    req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(s.bucketName),
        Key:    aws.String(key),
    }, func(opts *s3.PresignOptions) {
        opts.Expires = expiresIn
    })
    
    if err != nil {
        return "", fmt.Errorf("failed to generate presigned URL: %w", err)
    }
    
    return req.URL, nil
}

// FileExists checks if a file exists in S3
func (s *S3Service) FileExists(ctx context.Context, key string) (bool, int64, error) {
    result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: aws.String(s.bucketName),
        Key:    aws.String(key),
    })
    
    if err != nil {
        // File doesn't exist
        return false, 0, nil
    }
    
    return true, *result.ContentLength, nil
}

// DeleteFile deletes a file from S3
func (s *S3Service) DeleteFile(ctx context.Context, key string) error {
    _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
        Bucket: aws.String(s.bucketName),
        Key:    aws.String(key),
    })
    
    return err
}
```

#### 2.3: Implement gRPC Methods

```go
// internal/service/image_storage_service.go
package service

import (
    "context"
    "fmt"
    "time"
    
    pb "your-project/api/gen/v1"
    "your-project/internal/storage"
    "github.com/google/uuid"
)

type ImageStorageService struct {
    pb.UnimplementedImageStorageServiceServer
    s3Service *storage.S3Service
    db        *YourDB
}

// GetUploadUrl generates presigned upload URL
func (s *ImageStorageService) GetUploadUrl(
    ctx context.Context,
    req *pb.GetUploadUrlRequest,
) (*pb.GetUploadUrlResponse, error) {
    // 1. Extract user_id from JWT
    userID := getUserIDFromContext(ctx)
    
    // 2. Generate unique image_id
    imageID := uuid.New().String()
    
    // 3. Determine storage path
    // For new tools (tool_id=0): images/pending/{user_id}/{image_id}/{filename}
    // For existing tools: images/org_{org_id}/tool_{tool_id}/{image_id}/{filename}
    var storagePath string
    if req.ToolId == 0 {
        storagePath = fmt.Sprintf("images/pending/%s/%s/%s", 
            userID, imageID, req.Filename)
    } else {
        // Get org_id from tool
        tool, err := s.db.GetTool(ctx, req.ToolId)
        if err != nil {
            return nil, err
        }
        storagePath = fmt.Sprintf("images/org_%d/tool_%d/%s/%s", 
            tool.OrganizationID, req.ToolId, imageID, req.Filename)
    }
    
    // 4. Generate presigned PUT URL (15 minutes)
    uploadURL, err := s.s3Service.GeneratePresignedUploadURL(
        ctx,
        storagePath,
        req.ContentType,
        15*time.Minute,
    )
    if err != nil {
        return nil, err
    }
    
    // 5. Create pending image record in database
    err = s.db.CreatePendingImage(ctx, &PendingImage{
        ID:         imageID,
        ToolID:     req.ToolId,
        UserID:     userID,
        FileName:   req.Filename,
        FilePath:   storagePath,
        MimeType:   req.ContentType,
        IsPrimary:  req.IsPrimary,
        Status:     "PENDING",
        ExpiresAt:  time.Now().Add(15 * time.Minute),
    })
    if err != nil {
        return nil, err
    }
    
    // 6. Generate download URL (can use CloudFront CDN URL)
    downloadURL := fmt.Sprintf("https://cdn.your-domain.com/%s", storagePath)
    
    // 7. Return response
    return &pb.GetUploadUrlResponse{
        UploadUrl:   uploadURL,
        ImageId:     imageID,
        DownloadUrl: downloadURL,
        ExpiresAt:   time.Now().Add(15 * time.Minute).Unix(),
    }, nil
}

// ConfirmImageUpload verifies the upload and updates database
func (s *ImageStorageService) ConfirmImageUpload(
    ctx context.Context,
    req *pb.ConfirmImageUploadRequest,
) (*pb.ConfirmImageUploadResponse, error) {
    // 1. Extract user_id from JWT
    userID := getUserIDFromContext(ctx)
    
    // 2. Find pending image record
    pendingImage, err := s.db.GetPendingImage(ctx, req.ImageId, userID)
    if err != nil {
        return nil, fmt.Errorf("pending image not found")
    }
    
    // 3. Verify not already confirmed
    if pendingImage.Status == "CONFIRMED" {
        return nil, fmt.Errorf("image already confirmed")
    }
    
    // 4. Verify not expired
    if time.Now().After(pendingImage.ExpiresAt) {
        return nil, fmt.Errorf("upload URL expired")
    }
    
    // 5. Verify file exists in S3
    exists, fileSize, err := s.s3Service.FileExists(ctx, pendingImage.FilePath)
    if err != nil {
        return nil, err
    }
    if !exists {
        return nil, fmt.Errorf("image not found in storage. Please upload again.")
    }
    
    // 6. Update database
    err = s.db.ConfirmImage(ctx, &ConfirmImageParams{
        ImageID:   req.ImageId,
        ToolID:    req.ToolId,
        FileSize:  fileSize,
        Status:    "CONFIRMED",
        IsPrimary: pendingImage.IsPrimary,
    })
    if err != nil {
        return nil, err
    }
    
    // 7. If tool_id > 0, verify ownership and link to tool
    if req.ToolId > 0 {
        tool, err := s.db.GetTool(ctx, req.ToolId)
        if err != nil {
            return nil, err
        }
        if tool.OwnerID != userID {
            return nil, fmt.Errorf("permission denied")
        }
        
        // If this is primary, unset other primary images
        if pendingImage.IsPrimary {
            err = s.db.UnsetPrimaryImages(ctx, req.ToolId)
            if err != nil {
                return nil, err
            }
        }
    }
    
    // 8. Schedule async thumbnail generation
    go s.generateThumbnail(pendingImage.FilePath, imageID)
    
    // 9. Get complete ToolImage object
    toolImage, err := s.db.GetToolImage(ctx, req.ImageId)
    if err != nil {
        return nil, err
    }
    
    return &pb.ConfirmImageUploadResponse{
        Success:   true,
        ToolImage: toolImage,
        Message:   "Image uploaded successfully",
    }, nil
}

// GetDownloadUrl generates presigned download URL
func (s *ImageStorageService) GetDownloadUrl(
    ctx context.Context,
    req *pb.GetDownloadUrlRequest,
) (*pb.GetDownloadUrlResponse, error) {
    // 1. Extract user_id from JWT
    userID := getUserIDFromContext(ctx)
    
    // 2. Find image record
    image, err := s.db.GetToolImage(ctx, req.ImageId)
    if err != nil {
        return nil, err
    }
    
    // 3. Verify permissions
    tool, err := s.db.GetTool(ctx, req.ToolId)
    if err != nil {
        return nil, err
    }
    
    // Check if user can view this image
    canView := false
    if tool.OwnerID == userID {
        canView = true
    } else {
        // Check if user is in same org
        userOrgs, _ := s.db.GetUserOrganizations(ctx, userID)
        for _, org := range userOrgs {
            if org.ID == tool.OrganizationID {
                canView = true
                break
            }
        }
    }
    
    if !canView {
        return nil, fmt.Errorf("permission denied")
    }
    
    // 4. Determine file path
    filePath := image.FilePath
    if req.IsThumbnail && image.ThumbnailPath != "" {
        filePath = image.ThumbnailPath
    }
    
    // 5. Generate presigned GET URL (1 hour)
    downloadURL, err := s.s3Service.GeneratePresignedDownloadURL(
        ctx,
        filePath,
        1*time.Hour,
    )
    if err != nil {
        return nil, err
    }
    
    return &pb.GetDownloadUrlResponse{
        DownloadUrl: downloadURL,
        ExpiresAt:   time.Now().Add(1 * time.Hour).Unix(),
    }, nil
}
```

---

### Step 3: Database Schema Updates

```sql
-- Add new columns to tool_images table
ALTER TABLE tool_images ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'PENDING';
ALTER TABLE tool_images ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP;
ALTER TABLE tool_images ADD COLUMN IF NOT EXISTS mime_type VARCHAR(50);

-- Index for pending image cleanup
CREATE INDEX idx_tool_images_status_expires 
ON tool_images(status, expires_at) 
WHERE status = 'PENDING';

-- Cleanup job (run periodically)
DELETE FROM tool_images 
WHERE status = 'PENDING' 
  AND expires_at < NOW() - INTERVAL '1 day';
```

---

### Step 4: Thumbnail Generation (Async Job)

```go
// internal/worker/thumbnail_worker.go
package worker

import (
    "context"
    "image"
    "image/jpeg"
    "os"
    
    "github.com/nfnt/resize"
)

func (w *Worker) generateThumbnail(filePath string, imageID string) error {
    // 1. Download original image from S3
    file, err := w.s3Service.DownloadFile(context.Background(), filePath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    // 2. Decode image
    img, _, err := image.Decode(file)
    if err != nil {
        return err
    }
    
    // 3. Resize to 300x300 thumbnail
    thumbnail := resize.Thumbnail(300, 300, img, resize.Lanczos3)
    
    // 4. Encode as JPEG
    thumbnailPath := strings.Replace(filePath, "/images/", "/thumbnails/", 1)
    thumbnailFile, err := os.CreateTemp("", "thumbnail-*.jpg")
    if err != nil {
        return err
    }
    defer os.Remove(thumbnailFile.Name())
    
    err = jpeg.Encode(thumbnailFile, thumbnail, &jpeg.Options{Quality: 85})
    if err != nil {
        return err
    }
    
    // 5. Upload thumbnail to S3
    thumbnailFile.Seek(0, 0)
    err = w.s3Service.UploadFile(context.Background(), thumbnailPath, thumbnailFile)
    if err != nil {
        return err
    }
    
    // 6. Update database with thumbnail path
    return w.db.UpdateThumbnailPath(context.Background(), imageID, thumbnailPath)
}
```

---

## Migration Strategy

### Option 1: Clean Break (Recommended)

**Timeline:** Immediate

1. Deploy new presigned URL endpoints
2. Update Android client to use new endpoints
3. Mark streaming endpoints as deprecated
4. After 2 weeks, remove streaming endpoints

**Pros:**
- Clean codebase
- Better performance immediately
- Simpler to maintain

**Cons:**
- Older app versions stop working (force update required)

### Option 2: Parallel Support

**Timeline:** Gradual (2-3 months)

1. Deploy new presigned URL endpoints alongside streaming
2. Update Android client to use presigned URLs
3. Monitor usage of old endpoints
4. After 95% of traffic moved, remove old endpoints

**Pros:**
- No forced updates
- Gradual migration

**Cons:**
- Maintain two systems temporarily
- More complex codebase

---

## Cost Comparison

### Current: Streaming Through Server

```
Assumptions:
- 10,000 image uploads/month
- Average image size: 2 MB
- Server bandwidth: $0.08/GB
- Server CPU time: $0.0001/second

Monthly Cost:
- Bandwidth: 10,000 × 2 MB × $0.08/GB = $1.60
- CPU time: 10,000 × 2 seconds × $0.0001 = $2.00
- Total: $3.60/month

At 100,000 images/month: $36/month
```

### Presigned URLs + S3

```
Assumptions:
- 10,000 image uploads/month
- Average image size: 2 MB
- S3 storage: $0.023/GB/month
- S3 PUT requests: $0.005/1000 requests
- S3 GET requests: $0.0004/1000 requests
- CloudFront transfer: $0.085/GB (first 10 TB)

Monthly Cost:
- Storage: 20 GB × $0.023 = $0.46
- PUT requests: 10 × $0.005 = $0.05
- GET requests (10x downloads): 100 × $0.0004 = $0.04
- CloudFront: 20 GB × $0.085 = $1.70
- Server (URL generation): $0.10
- Total: $2.35/month

At 100,000 images/month: $23.50/month
```

**Savings: ~35% + Better performance + Infinite scalability**

---

## Security Considerations

### 1. URL Expiration

```go
// Upload URLs: Short expiration (15 minutes)
uploadExpiry := 15 * time.Minute

// Download URLs: Longer expiration (1 hour)
downloadExpiry := 1 * time.Hour

// Or use CloudFront signed URLs for even more control
```

### 2. Content Type Validation

```go
// Only allow image types
allowedTypes := []string{
    "image/jpeg",
    "image/png",
    "image/gif",
    "image/webp",
}

if !contains(allowedTypes, req.ContentType) {
    return nil, fmt.Errorf("invalid content type")
}
```

### 3. File Size Limits

```go
// In ConfirmImageUpload
if fileSize > 10*1024*1024 { // 10 MB
    return nil, fmt.Errorf("file too large")
}
```

### 4. Rate Limiting

```go
// Limit uploads per user
if s.rateLimiter.Exceeded(userID, "upload", 100, time.Hour) {
    return nil, fmt.Errorf("rate limit exceeded")
}
```

---

## Testing Strategy

### 1. Unit Tests

```go
func TestGeneratePresignedURL(t *testing.T) {
    service := NewImageStorageService(mockS3, mockDB)
    
    resp, err := service.GetUploadUrl(ctx, &pb.GetUploadUrlRequest{
        Filename:    "test.jpg",
        ContentType: "image/jpeg",
        ToolId:      123,
        IsPrimary:   true,
    })
    
    assert.NoError(t, err)
    assert.NotEmpty(t, resp.UploadUrl)
    assert.NotEmpty(t, resp.ImageId)
}
```

### 2. Integration Tests

```go
func TestCompleteUploadFlow(t *testing.T) {
    // 1. Get upload URL
    urlResp, _ := client.GetUploadUrl(ctx, &pb.GetUploadUrlRequest{...})
    
    // 2. Upload to presigned URL
    uploadFile(urlResp.UploadUrl, testImageBytes)
    
    // 3. Confirm upload
    confirmResp, _ := client.ConfirmImageUpload(ctx, &pb.ConfirmImageUploadRequest{
        ImageId:  urlResp.ImageId,
        ToolId:   123,
        FileSize: int64(len(testImageBytes)),
    })
    
    assert.True(t, confirmResp.Success)
    assert.NotNil(t, confirmResp.ToolImage)
}
```

### 3. Load Tests

```bash
# Simulate 1000 concurrent uploads
ab -n 1000 -c 100 -p upload_request.json \
   https://api.your-domain.com/v1/image/upload-url
```

---

## Monitoring & Observability

### Key Metrics

```go
// Prometheus metrics
var (
    uploadURLsGenerated = prometheus.NewCounter(
        "image_upload_urls_generated_total",
        "Total upload URLs generated",
    )
    
    uploadsConfirmed = prometheus.NewCounter(
        "image_uploads_confirmed_total",
        "Total uploads confirmed",
    )
    
    uploadFailures = prometheus.NewCounterVec(
        "image_upload_failures_total",
        "Upload failures by reason",
        []string{"reason"},
    )
    
    uploadDuration = prometheus.NewHistogram(
        "image_upload_duration_seconds",
        "Time from URL generation to confirmation",
    )
)
```

### CloudWatch/Logging

```go
log.WithFields(log.Fields{
    "user_id":    userID,
    "image_id":   imageID,
    "tool_id":    toolID,
    "file_size":  fileSize,
    "duration":   time.Since(startTime),
}).Info("Image upload completed")
```

---

## Recommendation: YES, Replace Streaming

### Summary

✅ **Replace streaming with presigned URL pattern**

**Reasons:**
1. **35% cost savings** + better performance
2. **Simpler code** - standard HTTP instead of gRPC streaming
3. **Infinite scalability** - cloud storage handles all traffic
4. **Industry standard** - used by AWS, Google, Microsoft, Dropbox, etc.
5. **Better user experience** - faster uploads/downloads

**Migration Path:**
- Deploy new endpoints alongside old ones
- Update Android client
- Monitor for 2 weeks
- Remove old streaming endpoints

**Estimated Effort:**
- Backend: 2-3 days
- Android: Already implemented ✅
- Testing: 2 days
- **Total: 1 week**

---

## Conclusion

The presigned URL pattern is the **modern, cloud-native approach** for file handling. It's simpler, faster, cheaper, and more scalable than streaming through your application server.

Your Android client is **already implemented** and ready to use this pattern. You just need to implement the backend endpoints following the business logic documentation.

**Next Steps:**
1. Review this document
2. Implement backend following `grpc_api_business_logic.md`
3. Test with Android client
4. Deploy and monitor

**Questions?** Refer to the business logic document for detailed step-by-step implementation.

---

**Document Location:** `docs/design/Serverside-docs/PRESIGNED_URL_PATTERN_EXPLAINED.md`  
**Related:** `docs/design/Serverside-docs/grpc_api_business_logic.md` (lines 412-550)
