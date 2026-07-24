package unit

import (
	"context"
	"errors"
	"testing"

	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newImageStorageServiceForTest() (service.ImageStorageService, *MockToolRepo, *MockStorage) {
	toolRepo := new(MockToolRepo)
	userRepo := new(MockUserRepo)
	orgRepo := new(MockOrganizationRepo)
	storageMock := new(MockStorage)
	svc := service.NewImageStorageService(toolRepo, userRepo, orgRepo, storageMock)
	return svc, toolRepo, storageMock
}

// TestImageStorageService_OwnershipChecks covers FR-004 (specs/006-tools-image-storage):
// ConfirmImageUpload, DeleteImage, and SetPrimaryImage must all require the caller to own the
// target tool (or, for ConfirmImageUpload, own the pending image). This is the same bug class
// as Known Discrepancy 1 (already fixed for UpdateTool/DeleteTool) recurring across three more
// RPCs in the same feature, found with zero coverage during SBR remediation.
func TestImageStorageService_OwnershipChecks(t *testing.T) {
	ctx := context.Background()
	const ownerID = int32(1)
	const otherUserID = int32(2)
	const toolID = int32(10)
	const imageID = int32(100)

	t.Run("ConfirmImageUpload rejects a caller who does not own the pending image", func(t *testing.T) {
		svc, toolRepo, _ := newImageStorageServiceForTest()
		toolRepo.On("GetImageByID", ctx, imageID).Return(&domain.ToolImage{ID: imageID, ToolID: toolID, UserID: ownerID, Status: "PENDING"}, nil)

		_, err := svc.ConfirmImageUpload(ctx, otherUserID, imageID, toolID, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		toolRepo.AssertNotCalled(t, "UpdateImage", mock.Anything, mock.Anything)
	})

	t.Run("ConfirmImageUpload accepts the actual owner", func(t *testing.T) {
		svc, toolRepo, storageMock := newImageStorageServiceForTest()
		toolRepo.On("GetImageByID", ctx, imageID).Return(&domain.ToolImage{ID: imageID, ToolID: toolID, UserID: ownerID, Status: "PENDING", FilePath: "path/to/file.jpg"}, nil)
		storageMock.On("FileExists", ctx, "path/to/file.jpg").Return(true, int64(1234), nil)
		toolRepo.On("GetImages", ctx, toolID).Return([]domain.ToolImage{}, nil)
		toolRepo.On("UpdateImage", ctx, mock.AnythingOfType("*domain.ToolImage")).Return(nil)
		// ConfirmImageUpload kicks off thumbnail generation in a fire-and-forget goroutine;
		// make it fail fast and harmlessly rather than racing this test's own assertions.
		storageMock.On("ReadFile", "path/to/file.jpg").Return(nil, errors.New("no thumbnail in this test")).Maybe()

		img, err := svc.ConfirmImageUpload(ctx, ownerID, imageID, toolID, 0)
		require.NoError(t, err)
		assert.Equal(t, "CONFIRMED", img.Status)
	})

	t.Run("DeleteImage rejects a caller who does not own the tool", func(t *testing.T) {
		svc, toolRepo, _ := newImageStorageServiceForTest()
		toolRepo.On("GetImageByID", ctx, imageID).Return(&domain.ToolImage{ID: imageID, ToolID: toolID}, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, OwnerID: ownerID}, nil)

		err := svc.DeleteImage(ctx, otherUserID, imageID, toolID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		toolRepo.AssertNotCalled(t, "DeleteImage", mock.Anything, mock.Anything)
	})

	// SEC-IMG-004 (sbr/rtm/009-security.rtm.md): DeleteImage verifies the caller owns toolID but
	// never checks that the fetched image actually belongs to toolID — unlike SetPrimaryImage
	// two functions below, which performs exactly this check (image.ToolID != toolID,
	// internal/service/image_storage.go:388). Any tool owner can delete any image on the
	// platform by pairing their own tool as the ownership anchor with a foreign image_id.
	t.Run("DeleteImage rejects an image that belongs to a different tool than toolID (SEC-IMG-004)", func(t *testing.T) {
		svc, toolRepo, storageMock := newImageStorageServiceForTest()
		const otherToolID = int32(20) // owned by the same caller, but the image below belongs to toolID, not otherToolID
		toolRepo.On("GetImageByID", ctx, imageID).Return(&domain.ToolImage{ID: imageID, ToolID: toolID}, nil)
		toolRepo.On("GetByID", ctx, otherToolID).Return(&domain.Tool{ID: otherToolID, OwnerID: ownerID}, nil)
		// These are only reached along the current (buggy) path, which proceeds straight to
		// deletion with no image.ToolID/toolID cross-check — allow them so the vulnerability
		// surfaces as a clean assertion failure below instead of an unrelated mock panic.
		storageMock.On("DeleteFile", ctx, mock.Anything).Maybe().Return(nil)
		toolRepo.On("DeleteImage", ctx, imageID).Maybe().Return(nil)

		err := svc.DeleteImage(ctx, ownerID, imageID, otherToolID)
		require.Error(t, err, "DeleteImage must reject when image.ToolID does not match the supplied toolID, the same way SetPrimaryImage already does")
	})

	t.Run("DeleteImage accepts the actual tool owner", func(t *testing.T) {
		svc, toolRepo, storageMock := newImageStorageServiceForTest()
		toolRepo.On("GetImageByID", ctx, imageID).Return(&domain.ToolImage{ID: imageID, ToolID: toolID, FilePath: "f.jpg"}, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, OwnerID: ownerID}, nil)
		storageMock.On("DeleteFile", ctx, "f.jpg").Return(nil)
		toolRepo.On("DeleteImage", ctx, imageID).Return(nil)

		err := svc.DeleteImage(ctx, ownerID, imageID, toolID)
		require.NoError(t, err)
	})

	t.Run("SetPrimaryImage rejects a caller who does not own the tool", func(t *testing.T) {
		svc, toolRepo, _ := newImageStorageServiceForTest()
		toolRepo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, OwnerID: ownerID}, nil)

		err := svc.SetPrimaryImage(ctx, otherUserID, toolID, imageID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		toolRepo.AssertNotCalled(t, "SetPrimaryImage", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("SetPrimaryImage accepts the actual tool owner", func(t *testing.T) {
		svc, toolRepo, _ := newImageStorageServiceForTest()
		toolRepo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, OwnerID: ownerID}, nil)
		toolRepo.On("GetImageByID", ctx, imageID).Return(&domain.ToolImage{ID: imageID, ToolID: toolID, Status: "CONFIRMED"}, nil)
		toolRepo.On("SetPrimaryImage", ctx, toolID, imageID).Return(nil)

		err := svc.SetPrimaryImage(ctx, ownerID, toolID, imageID)
		require.NoError(t, err)
	})
}

// TestImageStorageService_GetDownloadUrl covers FR-005 (specs/006-tools-image-storage):
// GetDownloadUrl must grant access to the tool's owner, or to any other caller only when the
// tool's status is AVAILABLE. Found with zero coverage of the actual disjunctive boundary
// during SBR remediation — every existing test used only the trivial owner-caller path.
func TestImageStorageService_GetDownloadUrl(t *testing.T) {
	ctx := context.Background()
	const ownerID = int32(1)
	const otherUserID = int32(2)
	const toolID = int32(10)
	const imageID = int32(100)
	image := domain.ToolImage{ID: imageID, ToolID: toolID, FilePath: "f.jpg"}

	t.Run("Owner can always download", func(t *testing.T) {
		svc, toolRepo, storageMock := newImageStorageServiceForTest()
		toolRepo.On("GetImages", ctx, toolID).Return([]domain.ToolImage{image}, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, OwnerID: ownerID, Status: domain.ToolStatusRented}, nil)
		storageMock.On("GeneratePresignedDownloadURL", ctx, "f.jpg", mock.Anything).Return("https://example.com/f.jpg", nil)

		_, _, err := svc.GetDownloadUrl(ctx, ownerID, imageID, toolID, false)
		require.NoError(t, err, "owner must always be able to download, regardless of tool status")
	})

	t.Run("Non-owner can download when the tool is AVAILABLE", func(t *testing.T) {
		svc, toolRepo, storageMock := newImageStorageServiceForTest()
		toolRepo.On("GetImages", ctx, toolID).Return([]domain.ToolImage{image}, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, OwnerID: ownerID, Status: domain.ToolStatusAvailable}, nil)
		storageMock.On("GeneratePresignedDownloadURL", ctx, "f.jpg", mock.Anything).Return("https://example.com/f.jpg", nil)

		_, _, err := svc.GetDownloadUrl(ctx, otherUserID, imageID, toolID, false)
		require.NoError(t, err, "a non-owner must be able to download when the tool is AVAILABLE")
	})

	t.Run("Non-owner is rejected when the tool is not AVAILABLE", func(t *testing.T) {
		svc, toolRepo, _ := newImageStorageServiceForTest()
		toolRepo.On("GetImages", ctx, toolID).Return([]domain.ToolImage{image}, nil)
		toolRepo.On("GetByID", ctx, toolID).Return(&domain.Tool{ID: toolID, OwnerID: ownerID, Status: domain.ToolStatusRented}, nil)

		_, _, err := svc.GetDownloadUrl(ctx, otherUserID, imageID, toolID, false)
		require.Error(t, err, "a non-owner must be rejected when the tool is not AVAILABLE")
		assert.Contains(t, err.Error(), "unauthorized")
	})
}
