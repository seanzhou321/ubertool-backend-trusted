package service

import (
	"context"
	"fmt"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type toolService struct {
	toolRepo repository.ToolRepository
	userRepo repository.UserRepository
	orgRepo  repository.OrganizationRepository
}

func NewToolService(toolRepo repository.ToolRepository, userRepo repository.UserRepository, orgRepo repository.OrganizationRepository) ToolService {
	return &toolService{
		toolRepo: toolRepo,
		userRepo: userRepo,
		orgRepo:  orgRepo,
	}
}

func (s *toolService) AddTool(ctx context.Context, tool *domain.Tool, images []string) error {
	if err := s.toolRepo.Create(ctx, tool); err != nil {
		return err
	}
	for i, url := range images {
		img := &domain.ToolImage{
			ToolID:        tool.ID,
			FileName:      url, // Use URL as filename for now
			FilePath:      url,
			ThumbnailPath: url,
			DisplayOrder:  int32(i),
		}
		if err := s.toolRepo.CreateImage(ctx, img); err != nil {
			return err
		}
	}
	return nil
}

func (s *toolService) GetTool(ctx context.Context, id, requestingUserID int32) (*domain.Tool, []domain.ToolImage, error) {
	tool, err := s.toolRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	images, err := s.toolRepo.GetImages(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	// Populate owner information
	if err := s.populateToolOwner(ctx, tool, requestingUserID); err != nil {
		// Log error but don't fail the request
	}

	return tool, images, nil
}

func (s *toolService) UpdateTool(ctx context.Context, tool *domain.Tool) error {
	return s.toolRepo.Update(ctx, tool)
}

func (s *toolService) DeleteTool(ctx context.Context, id int32) error {
	return s.toolRepo.Delete(ctx, id)
}

func (s *toolService) ListTools(ctx context.Context, orgID, requestingUserID int32, page, pageSize int32) ([]domain.Tool, int32, error) {
	tools, count, err := s.toolRepo.ListByOrg(ctx, orgID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	// Populate owner information for each tool
	for i := range tools {
		if err := s.populateToolOwner(ctx, &tools[i], requestingUserID); err != nil {
			// Log error but don't fail the request
			continue
		}
	}

	return tools, count, nil
}

func (s *toolService) ListMyTools(ctx context.Context, userID int32, page, pageSize int32) ([]domain.Tool, int32, error) {
	return s.toolRepo.ListByOwner(ctx, userID, page, pageSize)
}

func (s *toolService) SearchTools(ctx context.Context, userID, orgID int32, query string, categories []string, maxPrice int32, condition string, page, pageSize int32) ([]domain.Tool, int32, error) {
	if orgID != 0 {
		// verify user belongs to this organization
		// Assuming GetUserOrg checks existence/active status
		_, err := s.userRepo.GetUserOrg(ctx, userID, orgID)
		if err != nil {
			return nil, 0, fmt.Errorf("user does not belong to organization %d: %w", orgID, err)
		}
	}
	tools, count, err := s.toolRepo.Search(ctx, userID, orgID, query, categories, maxPrice, condition, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	// Populate owner information with shared organizations
	for i := range tools {
		if err := s.populateToolOwner(ctx, &tools[i], userID); err != nil {
			fmt.Printf("ERROR: Fail to retrieve Tool ID %d owner (ID %d).\n", tools[i].ID, tools[i].OwnerID)
			continue
		}
		// Check that shared organizations are not empty for SearchTools
		if tools[i].Owner != nil && len(tools[i].Owner.Orgs) == 0 {
			fmt.Printf("ERROR: Tool ID %d owner (ID %d) has no shared organizations with requesting user (ID %d). This should not happen in SearchTools.\n",
				tools[i].ID, tools[i].OwnerID, userID)
		}
	}

	return tools, count, nil
}

func (s *toolService) populateToolOwner(ctx context.Context, tool *domain.Tool, requestingUserID int32) error {
	// Get the owner user details
	owner, err := s.userRepo.GetByID(ctx, tool.OwnerID)
	if err != nil {
		return err
	}

	// Get organizations that both the owner and requesting user share
	sharedOrgs, err := s.getSharedOrganizations(ctx, tool.OwnerID, requestingUserID)
	if err != nil {
		return err
	}

	owner.Orgs = sharedOrgs
	tool.Owner = owner
	return nil
}

func (s *toolService) getSharedOrganizations(ctx context.Context, ownerID, requestingUserID int32) ([]domain.Organization, error) {
	// Get all organizations for the owner
	ownerOrgs, err := s.userRepo.ListUserOrgs(ctx, ownerID)
	if err != nil {
		return nil, err
	}

	// Get all organizations for the requesting user
	requestingUserOrgs, err := s.userRepo.ListUserOrgs(ctx, requestingUserID)
	if err != nil {
		return nil, err
	}

	// Create a map of requesting user's org IDs for fast lookup
	requestingOrgIDs := make(map[int32]bool)
	for _, userOrg := range requestingUserOrgs {
		requestingOrgIDs[userOrg.OrgID] = true
	}

	// Find shared organizations
	var sharedOrgs []domain.Organization
	for _, ownerOrg := range ownerOrgs {
		if requestingOrgIDs[ownerOrg.OrgID] {
			// This is a shared organization, fetch its details
			org, err := s.orgRepo.GetByID(ctx, ownerOrg.OrgID)
			if err != nil {
				continue // Skip if we can't fetch org details
			}
			sharedOrgs = append(sharedOrgs, *org)
		}
	}

	return sharedOrgs, nil
}

func (s *toolService) ListCategories(ctx context.Context) ([]string, error) {
	// Static list for now, or could be fetched from DB
	return []string{"Hand Tools", "Power Tools", "Gardening", "Plumbing", "Electrical", "Automotive", "Painting", "Cleaning"}, nil
}
