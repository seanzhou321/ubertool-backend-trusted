package grpc

import (
	"context"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/service"
)

type ImageStorageHandler struct {
	pb.UnimplementedImageStorageServiceServer
	storeSvc service.ImageStorageService
}

func NewImageStorageHandler(storeSvc service.ImageStorageService) *ImageStorageHandler {
	return &ImageStorageHandler{storeSvc: storeSvc}
}

// GetUploadUrl generates a presigned URL for uploading an image
func (h *ImageStorageHandler) GetUploadUrl(ctx context.Context, req *pb.GetUploadUrlRequest) (*pb.GetUploadUrlResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	image, uploadURL, downloadURL, expiresAt, err := h.storeSvc.GetUploadUrl(
		ctx,
		userID,
		req.Filename,
		req.ContentType,
		req.ToolId,
		req.IsPrimary,
	)
	if err != nil {
		return nil, err
	}

	return &pb.GetUploadUrlResponse{
		UploadUrl:   uploadURL,
		ImageId:     image.ID,
		DownloadUrl: downloadURL,
		ExpiresAt:   expiresAt,
	}, nil
}

// ConfirmImageUpload confirms that an image was successfully uploaded
func (h *ImageStorageHandler) ConfirmImageUpload(ctx context.Context, req *pb.ConfirmImageUploadRequest) (*pb.ConfirmImageUploadResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	image, err := h.storeSvc.ConfirmImageUpload(ctx, userID, req.ImageId, req.ToolId, req.FileSize)
	if err != nil {
		return nil, err
	}

	return &pb.ConfirmImageUploadResponse{
		Success:   true,
		ToolImage: MapDomainToolImageToProto(image),
		Message:   "Image uploaded successfully",
	}, nil
}

// GetDownloadUrl generates a presigned URL for downloading an image
func (h *ImageStorageHandler) GetDownloadUrl(ctx context.Context, req *pb.GetDownloadUrlRequest) (*pb.GetDownloadUrlResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	downloadURL, expiresAt, err := h.storeSvc.GetDownloadUrl(ctx, userID, req.ImageId, req.ToolId, req.IsThumbnail)
	if err != nil {
		return nil, err
	}

	return &pb.GetDownloadUrlResponse{
		DownloadUrl: downloadURL,
		ExpiresAt:   expiresAt,
	}, nil
}

// GetToolImages retrieves all images for a tool
func (h *ImageStorageHandler) GetToolImages(ctx context.Context, req *pb.GetToolImagesRequest) (*pb.GetToolImagesResponse, error) {
	images, err := h.storeSvc.GetToolImages(ctx, req.ToolId)
	if err != nil {
		return nil, err
	}

	protoImages := make([]*pb.ToolImage, len(images))
	for i, img := range images {
		protoImages[i] = MapDomainToolImageToProto(&img)
	}

	return &pb.GetToolImagesResponse{Images: protoImages}, nil
}

// DeleteImage deletes an image
func (h *ImageStorageHandler) DeleteImage(ctx context.Context, req *pb.DeleteImageRequest) (*pb.DeleteImageResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	err = h.storeSvc.DeleteImage(ctx, userID, req.ImageId, req.ToolId)
	if err != nil {
		return nil, err
	}

	return &pb.DeleteImageResponse{Success: true}, nil
}

// SetPrimaryImage sets an image as the primary image for a tool
func (h *ImageStorageHandler) SetPrimaryImage(ctx context.Context, req *pb.SetPrimaryImageRequest) (*pb.SetPrimaryImageResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	err = h.storeSvc.SetPrimaryImage(ctx, userID, req.ToolId, req.ImageId)
	if err != nil {
		return nil, err
	}

	return &pb.SetPrimaryImageResponse{
		Success: true,
		Message: "Primary image updated successfully",
	}, nil
}
