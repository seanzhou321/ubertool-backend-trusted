package grpc

import (
	"context"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/service"
)

type OrganizationHandler struct {
	pb.UnimplementedOrganizationServiceServer
	orgSvc service.OrganizationService
}

func NewOrganizationHandler(orgSvc service.OrganizationService) *OrganizationHandler {
	return &OrganizationHandler{orgSvc: orgSvc}
}

func (h *OrganizationHandler) ListMyOrganizations(ctx context.Context, req *pb.ListMyOrganizationsRequest) (*pb.ListOrganizationsResponse, error) {
	orgs, err := h.orgSvc.ListOrganizations(ctx)
	if err != nil {
		return nil, err
	}
	protoOrgs := make([]*pb.Organization, len(orgs))
	for i, o := range orgs {
		protoOrgs[i] = MapDomainOrgToProto(&o)
	}
	return &pb.ListOrganizationsResponse{Organizations: protoOrgs}, nil
}

func (h *OrganizationHandler) GetOrganization(ctx context.Context, req *pb.GetOrganizationRequest) (*pb.GetOrganizationResponse, error) {
	org, err := h.orgSvc.GetOrganization(ctx, req.OrganizationId)
	if err != nil {
		return nil, err
	}
	return &pb.GetOrganizationResponse{Organization: MapDomainOrgToProto(org)}, nil
}

func (h *OrganizationHandler) SearchOrganizations(ctx context.Context, req *pb.SearchOrganizationsRequest) (*pb.ListOrganizationsResponse, error) {
	orgs, err := h.orgSvc.SearchOrganizations(ctx, req.Name, req.Metro)
	if err != nil {
		return nil, err
	}
	protoOrgs := make([]*pb.Organization, len(orgs))
	for i, o := range orgs {
		protoOrgs[i] = MapDomainOrgToProto(&o)
	}
	return &pb.ListOrganizationsResponse{Organizations: protoOrgs}, nil
}
