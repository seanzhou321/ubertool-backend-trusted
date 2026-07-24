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

// maxToolPriceCents is the proto contract's documented requirement: "Prices stored as cents
// (int32 max = $21M, requirement max = $1000)" — see api/proto/.../tool_service.proto:67,90,143.
// Enforced server-side here since none of AddTool/UpdateTool previously validated it at all (see
// sbr/rtm/009-security.rtm.md SEC-TOOL-004): a malicious owner could set a negative or
// arbitrarily large price, which flows directly into rental cost and settlement math.
const maxToolPriceCents = 100_000 // $1000.00

// validateToolPrices rejects a negative or above-max price on any of the 4 fields, and
// additionally requires the 3 rental-rate fields to be strictly positive — CalculateRentalCost
// (internal/utils) caps each shorter tier's cost at the next tier's price (e.g. day cost capped
// at week price), so a zero week/month price would silently zero out rental costs regardless of
// the rental's actual duration unit. replacement_cost_cents has no such dependency and 0 is a
// legitimate "not specified" value for it, so it is only bounds-checked, not required positive.
func validateToolPrices(tool *domain.Tool) error {
	rentalRates := map[string]int32{
		"price_per_day_cents":   tool.PricePerDayCents,
		"price_per_week_cents":  tool.PricePerWeekCents,
		"price_per_month_cents": tool.PricePerMonthCents,
	}
	for name, cents := range rentalRates {
		if cents <= 0 {
			return fmt.Errorf("%s must be a positive amount", name)
		}
		if cents > maxToolPriceCents {
			return fmt.Errorf("%s exceeds the maximum allowed value of %d cents ($1000)", name, maxToolPriceCents)
		}
	}
	if tool.ReplacementCostCents < 0 || tool.ReplacementCostCents > maxToolPriceCents {
		return fmt.Errorf("replacement_cost_cents must be between 0 and %d cents ($1000)", maxToolPriceCents)
	}
	return nil
}

func (s *toolService) AddTool(ctx context.Context, tool *domain.Tool, images []string) error {
	if err := validateToolPrices(tool); err != nil {
		return err
	}
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

	// SEC-TOOL-002 (sbr/rtm/009-security.rtm.md): GetTool must be scoped like its siblings
	// SearchTools/ListTools — reject a caller who neither owns the tool nor shares an org with
	// its owner, rather than returning full tool detail (including the owner's email/phone) to
	// any authenticated user platform-wide.
	if requestingUserID != tool.OwnerID {
		sharedOrgs, err := s.getSharedOrganizations(ctx, tool.OwnerID, requestingUserID)
		if err != nil {
			return nil, nil, err
		}
		if len(sharedOrgs) == 0 {
			return nil, nil, fmt.Errorf("unauthorized: you do not share an organization with this tool's owner")
		}
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

func (s *toolService) UpdateTool(ctx context.Context, callerID int32, tool *domain.Tool) error {
	existing, err := s.toolRepo.GetByID(ctx, tool.ID)
	if err != nil {
		return err
	}
	if existing.OwnerID != callerID {
		return fmt.Errorf("unauthorized: only the tool owner may update this tool")
	}
	if err := validateToolPrices(tool); err != nil {
		return err
	}
	return s.toolRepo.Update(ctx, tool)
}

func (s *toolService) DeleteTool(ctx context.Context, callerID, id int32) error {
	existing, err := s.toolRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.OwnerID != callerID {
		return fmt.Errorf("unauthorized: only the tool owner may delete this tool")
	}
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

func (s *toolService) SearchTools(ctx context.Context, userID, orgID int32, metro, query string, categories []string, maxPrice int32, condition string, page, pageSize int32) ([]domain.Tool, int32, error) {
	fmt.Printf("DEBUG SearchTools: userID=%d, orgID=%d, metro=%q, query=%q, categories=%v, maxPrice=%d, condition=%q, page=%d, pageSize=%d\n",
		userID, orgID, metro, query, categories, maxPrice, condition, page, pageSize)

	// Validate required parameters
	if query == "" {
		fmt.Printf("ERROR SearchTools: query parameter is empty or missing\n")
		return nil, 0, fmt.Errorf("query parameter is required and cannot be empty")
	}

	// Get metro from org if orgID is provided, otherwise metro must be specified
	var searchMetro string
	if orgID != 0 {
		fmt.Printf("DEBUG SearchTools: orgID provided (%d), fetching metro from organization\n", orgID)
		// verify user belongs to this organization
		_, err := s.userRepo.GetUserOrg(ctx, userID, orgID)
		if err != nil {
			return nil, 0, fmt.Errorf("user does not belong to organization %d: %w", orgID, err)
		}
		// Get metro from organization
		org, err := s.orgRepo.GetByID(ctx, orgID)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get organization: %w", err)
		}
		searchMetro = org.Metro
	} else {
		// When orgID not provided, metro must be specified
		fmt.Printf("DEBUG SearchTools: orgID=0, using metro from request parameter\n")
		if metro == "" {
			fmt.Printf("ERROR SearchTools: metro parameter is empty when orgID=0\n")
			return nil, 0, fmt.Errorf("metro parameter is required when organization_id is not specified")
		}
		searchMetro = metro
	}
	fmt.Printf("DEBUG SearchTools: searchMetro=%q\n", searchMetro)

	// Default condition to exclude damaged tools if not specified
	if condition == "" {
		condition = "NOT_DAMAGED"
		fmt.Printf("DEBUG SearchTools: condition defaulted to NOT_DAMAGED\n")
	}

	fmt.Printf("DEBUG SearchTools: calling repository Search with metro=%q, query=%q, condition=%q\n", searchMetro, query, condition)
	tools, count, err := s.toolRepo.Search(ctx, userID, searchMetro, query, categories, maxPrice, condition, page, pageSize)
	if err != nil {
		fmt.Printf("ERROR SearchTools: repository Search failed: %v\n", err)
		return nil, 0, err
	}
	fmt.Printf("DEBUG SearchTools: repository returned %d tools (count=%d)\n", len(tools), count)

	// Populate owner information with shared organizations and filter out tools with no shared orgs
	var filteredTools []domain.Tool
	fmt.Printf("DEBUG SearchTools: Processing %d tools for owner population and filtering\n", len(tools))
	for i := range tools {
		fmt.Printf("DEBUG SearchTools: Processing tool ID=%d, ownerID=%d\n", tools[i].ID, tools[i].OwnerID)
		if err := s.populateToolOwner(ctx, &tools[i], userID); err != nil {
			fmt.Printf("ERROR: Fail to retrieve Tool ID %d owner (ID %d): %v\n", tools[i].ID, tools[i].OwnerID, err)
			continue
		}

		// Check owner and orgs
		if tools[i].Owner == nil {
			fmt.Printf("WARNING: Tool ID %d has nil Owner after populateToolOwner\n", tools[i].ID)
			continue
		}

		fmt.Printf("DEBUG SearchTools: Tool ID=%d owner populated: ownerID=%d, ownerName=%q, orgs count=%d\n",
			tools[i].ID, tools[i].Owner.ID, tools[i].Owner.Name, len(tools[i].Owner.Orgs))

		// Filter out tools where owner has no shared organizations with requesting user
		if len(tools[i].Owner.Orgs) == 0 {
			fmt.Printf("FILTERED OUT: Tool ID %d owner (ID %d) has no shared organizations with requesting user (ID %d).\n",
				tools[i].ID, tools[i].OwnerID, userID)
			continue
		}

		fmt.Printf("DEBUG SearchTools: Tool ID=%d INCLUDED in results\n", tools[i].ID)
		filteredTools = append(filteredTools, tools[i])
	}

	fmt.Printf("DEBUG SearchTools: Returning %d tools after filtering (originally %d)\n", len(filteredTools), len(tools))
	return filteredTools, int32(len(filteredTools)), nil
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
		fmt.Printf("DEBUG: Failed to get owner orgs for user %d: %v\n", ownerID, err)
		return nil, err
	}
	fmt.Printf("DEBUG: Owner (user %d) orgs: %+v\n", ownerID, ownerOrgs)

	// Get all organizations for the requesting user
	requestingUserOrgs, err := s.userRepo.ListUserOrgs(ctx, requestingUserID)
	if err != nil {
		fmt.Printf("DEBUG: Failed to get requesting user orgs for user %d: %v\n", requestingUserID, err)
		return nil, err
	}
	fmt.Printf("DEBUG: Requesting user (user %d) orgs: %+v\n", requestingUserID, requestingUserOrgs)

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
				fmt.Printf("DEBUG: Failed to get org details for org %d: %v\n", ownerOrg.OrgID, err)
				continue // Skip if we can't fetch org details
			}
			sharedOrgs = append(sharedOrgs, *org)
		}
	}

	fmt.Printf("DEBUG: Shared orgs between user %d and %d: %+v\n", ownerID, requestingUserID, sharedOrgs)
	return sharedOrgs, nil
}

func (s *toolService) ListCategories(ctx context.Context) ([]string, error) {
	// Static list for now, or could be fetched from DB
	return []string{"Hand Tools", "Power Tools", "Gardening", "Plumbing", "Electrical", "Automotive", "Painting", "Cleaning"}, nil
}
