package grpc

import (
	"context"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GetUserIDFromContext extracts the user ID from the gRPC metadata.
// It expects a header named "user-id".
func GetUserIDFromContext(ctx context.Context) (int32, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, status.Errorf(codes.Unauthenticated, "metadata is not provided")
	}

	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return 0, status.Errorf(codes.Unauthenticated, "user_id is not provided in metadata")
	}

	userIDStr := userIDs[0]
	userID, err := strconv.ParseInt(userIDStr, 10, 32)
	if err != nil {
		return 0, status.Errorf(codes.InvalidArgument, "invalid user_id format: %v", err)
	}

	return int32(userID), nil
}
