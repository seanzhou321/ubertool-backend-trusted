package service

import (
	"context"
	"ubertool-backend-trusted/internal/domain"
	"ubertool-backend-trusted/internal/repository"
)

type userService struct {
	userRepo repository.UserRepository
	orgRepo  repository.OrganizationRepository
}

func NewUserService(userRepo repository.UserRepository, orgRepo repository.OrganizationRepository) UserService {
	return &userService{
		userRepo: userRepo,
		orgRepo:  orgRepo,
	}
}

func (s *userService) GetUserProfile(ctx context.Context, userID int32) (*domain.User, []domain.Organization, []domain.UserOrg, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	userOrgs, err := s.userRepo.ListUserOrgs(ctx, userID)
	if err != nil {
		return nil, nil, nil, err
	}

	var orgs []domain.Organization
	for _, uo := range userOrgs {
		org, err := s.orgRepo.GetByID(ctx, uo.OrgID)
		if err != nil {
			continue
		}
		if org != nil {
			orgs = append(orgs, *org)
		}
	}
	return user, orgs, userOrgs, nil
}

func (s *userService) UpdateProfile(ctx context.Context, userID int32, name, email, phone, avatarURL string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	user.Name = name
	user.Email = email
	user.PhoneNumber = phone
	user.AvatarURL = avatarURL
	return s.userRepo.Update(ctx, user)
}
