package service

import (
	"context"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) GetUserProfile(ctx context.Context, userID int32) (*domain.User, []domain.UserOrg, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	orgs, err := s.userRepo.ListUserOrgs(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	return user, orgs, nil
}

func (s *userService) UpdateProfile(ctx context.Context, userID int32, name, avatarURL string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	user.Name = name
	user.AvatarURL = avatarURL
	return s.userRepo.Update(ctx, user)
}
