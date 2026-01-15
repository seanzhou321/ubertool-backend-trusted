package service

import (
	"context"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type toolService struct {
	toolRepo repository.ToolRepository
}

func NewToolService(toolRepo repository.ToolRepository) ToolService {
	return &toolService{toolRepo: toolRepo}
}

func (s *toolService) AddTool(ctx context.Context, tool *domain.Tool, images []string) error {
	if err := s.toolRepo.Create(ctx, tool); err != nil {
		return err
	}
	for i, url := range images {
		img := &domain.ToolImage{
			ToolID:       tool.ID,
			ImageURL:     url,
			DisplayOrder: int32(i),
		}
		if err := s.toolRepo.AddImage(ctx, img); err != nil {
			return err
		}
	}
	return nil
}

func (s *toolService) GetTool(ctx context.Context, id int32) (*domain.Tool, []domain.ToolImage, error) {
	tool, err := s.toolRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	images, err := s.toolRepo.GetImages(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return tool, images, nil
}

func (s *toolService) UpdateTool(ctx context.Context, tool *domain.Tool) error {
	return s.toolRepo.Update(ctx, tool)
}

func (s *toolService) DeleteTool(ctx context.Context, id int32) error {
	return s.toolRepo.Delete(ctx, id)
}

func (s *toolService) ListTools(ctx context.Context, orgID int32, page, pageSize int32) ([]domain.Tool, int32, error) {
	return s.toolRepo.ListByOrg(ctx, orgID, page, pageSize)
}

func (s *toolService) SearchTools(ctx context.Context, orgID int32, query string, categories []string, maxPrice int32, condition string, page, pageSize int32) ([]domain.Tool, int32, error) {
	return s.toolRepo.Search(ctx, orgID, query, categories, maxPrice, condition, page, pageSize)
}

func (s *toolService) ListCategories(ctx context.Context) ([]string, error) {
	// Static list for now, or could be fetched from DB
	return []string{"Hand Tools", "Power Tools", "Gardening", "Plumbing", "Electrical", "Automotive", "Painting", "Cleaning"}, nil
}
