package storage

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3StorageService implements StorageInterface using AWS S3 presigned URLs.
// The EC2 instance must have an IAM role with s3:PutObject, s3:GetObject,
// s3:DeleteObject, and s3:HeadObject on the bucket.
type S3StorageService struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

// NewS3StorageService creates an S3StorageService using the credential chain
// (EC2 instance profile → env vars → ~/.aws/credentials).
func NewS3StorageService(ctx context.Context, bucket, region string) (*S3StorageService, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg)
	return &S3StorageService{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    bucket,
	}, nil
}

// GeneratePresignedUploadURL returns an S3 presigned PUT URL.
func (s *S3StorageService) GeneratePresignedUploadURL(
	ctx context.Context,
	key string,
	contentType string,
	expiresIn time.Duration,
) (string, error) {
	req, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// GeneratePresignedDownloadURL returns an S3 presigned GET URL.
func (s *S3StorageService) GeneratePresignedDownloadURL(
	ctx context.Context,
	key string,
	expiresIn time.Duration,
) (string, error) {
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// FileExists checks whether a key exists in S3 and returns its size.
func (s *S3StorageService) FileExists(ctx context.Context, key string) (bool, int64, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// AWS SDK v2 wraps 404 as a *smithy.ResponseError / NoSuchKey — treat any
		// error as "not found" rather than a hard failure so callers can handle it.
		return false, 0, nil
	}
	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return true, size, nil
}

// DeleteFile removes an object from S3.
func (s *S3StorageService) DeleteFile(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

// SaveFile is not used by S3 storage (uploads go directly from client to S3
// via presigned URL). It is only present to satisfy StorageInterface.
func (s *S3StorageService) SaveFile(key string, reader io.Reader) error {
	ctx := context.Background()
	body, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	})
	return err
}

// ReadFile is not used by S3 storage (downloads go directly from S3 via
// presigned URL). It is only present to satisfy StorageInterface.
func (s *S3StorageService) ReadFile(key string) (io.ReadCloser, error) {
	ctx := context.Background()
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}
