package grpc

import (
	"context"
	"fmt"

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
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	orgs, userOrgs, err := h.orgSvc.ListMyOrganizations(ctx, userID)
	if err != nil {
		return nil, err
	}
	protoOrgs := make([]*pb.Organization, len(orgs))
	for i, o := range orgs {
		// Find matching userOrg to get role and balance
		var userRole string
		for _, uo := range userOrgs {
			if uo.OrgID == o.ID {
				userRole = string(uo.Role)
				protoOrgs[i] = MapDomainOrgToProto(&o, userRole)
				protoOrgs[i].UserBalance = uo.BalanceCents
				break
			}
		}
		if userRole == "" {
			// Fallback in case userOrg not found
			protoOrgs[i] = MapDomainOrgToProto(&o, "")
		}
	}
	return &pb.ListOrganizationsResponse{Organizations: protoOrgs}, nil
}

func (h *OrganizationHandler) GetOrganization(ctx context.Context, req *pb.GetOrganizationRequest) (*pb.GetOrganizationResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	org, userOrg, err := h.orgSvc.GetOrganization(ctx, req.OrganizationId, userID)
	if err != nil {
		return nil, err
	}
	var userRole string
	if userOrg != nil {
		userRole = string(userOrg.Role)
	}
	return &pb.GetOrganizationResponse{Organization: MapDomainOrgToProto(org, userRole)}, nil
}

func (h *OrganizationHandler) SearchOrganizations(ctx context.Context, req *pb.SearchOrganizationsRequest) (*pb.ListOrganizationsResponse, error) {
	orgs, err := h.orgSvc.SearchOrganizations(ctx, req.Name, req.Metro)
	if err != nil {
		return nil, err
	}
	protoOrgs := make([]*pb.Organization, len(orgs))
	for i, o := range orgs {
		// SearchOrganizations should not populate user_role
		protoOrgs[i] = MapDomainOrgToProto(&o, "")
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
	return &pb.UpdateOrganizationResponse{Organization: MapDomainOrgToProto(org, "")}, nil
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
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	err = h.orgSvc.CreateOrganization(ctx, userID, org)
	if err != nil {
		return nil, err
	}
	return &pb.CreateOrganizationResponse{Organization: MapDomainOrgToProto(org, "")}, nil
}

func (h *OrganizationHandler) JoinOrganizationWithInvite(ctx context.Context, req *pb.JoinOrganizationRequest) (*pb.JoinOrganizationResponse, error) {
	userID, err := GetUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	org, _, err := h.orgSvc.JoinOrganizationWithInvite(ctx, userID, req.InvitationCode)
	if err != nil {
		return nil, err
	}

	message := fmt.Sprintf("Successfully joined %s", org.Name)
	return &pb.JoinOrganizationResponse{
		Success:      true,
		Organization: MapDomainOrgToProto(org, ""),
		Message:      message,
	}, nil
}
