package grpc

import (
	"context"
	"io"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"
)

type ImageStorageHandler struct {
	pb.UnimplementedImageStorageServiceServer
	storeSvc service.ImageStorageService
}

func NewImageStorageHandler(storeSvc service.ImageStorageService) *ImageStorageHandler {
	return &ImageStorageHandler{storeSvc: storeSvc}
}

func (h *ImageStorageHandler) UploadImage(stream pb.ImageStorageService_UploadImageServer) error {
	var toolID int32
	var filename string
	var mimeType string
	var fileBytes []byte

	// Read first chunk to get metadata
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	if req.UploadImageRequestObject == nil {
		return io.ErrUnexpectedEOF // First msg must include metadata
	}
	
	toolID = req.UploadImageRequestObject.ToolId
	filename = req.UploadImageRequestObject.FileName
	mimeType = req.UploadImageRequestObject.MimeType
	
	// Append first chunk data if any
	fileBytes = append(fileBytes, req.Chunk...)

	// Read remaining chunks
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		fileBytes = append(fileBytes, req.Chunk...)
	}

	// Call service
	img, err := h.storeSvc.UploadImage(stream.Context(), toolID, fileBytes, filename, mimeType)
	if err != nil {
		return err
	}

	// Send response
	return stream.SendAndClose(&pb.UploadImageResponse{
		ToolImage: MapDomainToolImageToProto(img),
	})
}

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

func (h *ImageStorageHandler) DownloadImage(req *pb.DownloadImageRequest, stream pb.ImageStorageService_DownloadImageServer) error {
	data, _, err := h.storeSvc.DownloadImage(stream.Context(), req.ToolId, req.ImageId)
	if err != nil {
		return err
	}
	
	// We need to fetch the image metadata to return map to proto
	// This is inefficient (double fetch) but without changing service signature further it's reliable.
	// Actually, DownloadImage service could return *domain.ToolImage but we didn't change it.
	// Let's rely on DownloadImage service returning data.
	// We need to construct a partial ToolImage or fetch it.
	// Let's fetch it for correctness.
	images, err := h.storeSvc.GetToolImages(stream.Context(), req.ToolId)
	if err != nil {
		return err
	}
	var targetImg *domain.ToolImage
	for _, img := range images {
		if img.ID == req.ImageId {
			targetImg = &img
			break
		}
	}
	// If not found, DownloadImage would have failed already, but safe to check.

	chunkSize := 64 * 1024 // 64KB
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		
		resp := &pb.DownloadImageResponse{
			Chunk: data[i:end],
		}
		
		if i == 0 && targetImg != nil {
			resp.ToolImage = MapDomainToolImageToProto(targetImg)
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
	return nil
}

func (h *ImageStorageHandler) DeleteImage(ctx context.Context, req *pb.DeleteImageRequest) (*pb.DeleteImageResponse, error) {
	err := h.storeSvc.DeleteImage(ctx, req.ImageId)
	if err != nil {
		return nil, err
	}
	return &pb.DeleteImageResponse{Success: true}, nil
}

func (h *ImageStorageHandler) SetPrimaryImage(ctx context.Context, req *pb.SetPrimaryImageRequest) (*pb.SetPrimaryImageResponse, error) {
	err := h.storeSvc.SetPrimaryImage(ctx, req.ToolId, req.ImageId)
	if err != nil {
		return nil, err
	}
	return &pb.SetPrimaryImageResponse{Success: true, Message: "Primary image updated"}, nil
}
