package grpc

import (
	"context"

	pb "ubertool-backend-trusted/api/gen/v1"
	"ubertool-backend-trusted/internal/domain"
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
	_, err := GetUserIDFromContext(ctx) // userID not used yet in service, but check auth
	if err != nil {
		return nil, err
	}
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
func (h *OrganizationHandler) UpdateOrganization(ctx context.Context, req *pb.UpdateOrganizationRequest) (*pb.UpdateOrganizationResponse, error) {
	org := &domain.Organization{
		ID:               req.OrganizationId,
		Name:             req.Name,
		Description:      req.Description,
		Address:          req.Address,
		Metro:            req.Metro,
		AdminEmail:       req.AdminEmail,
		AdminPhoneNumber: req.AdminPhone,
	}
	err := h.orgSvc.UpdateOrganization(ctx, org)
	if err != nil {
		return nil, err
	}
	return &pb.UpdateOrganizationResponse{Organization: MapDomainOrgToProto(org)}, nil
}
func (h *OrganizationHandler) CreateOrganization(ctx context.Context, req *pb.CreateOrganizationRequest) (*pb.CreateOrganizationResponse, error) {
	org := &domain.Organization{
		Name:             req.Name,
		Description:      req.Description,
		Address:          req.Address,
		Metro:            req.Metro,
		AdminEmail:       req.AdminEmail,
		AdminPhoneNumber: req.AdminPhone,
	}
	err := h.orgSvc.CreateOrganization(ctx, org)
	if err != nil {
		return nil, err
	}
	return &pb.CreateOrganizationResponse{Organization: MapDomainOrgToProto(org)}, nil
}
