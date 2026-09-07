package user

import "context"

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) CreateUser(ctx context.Context, name string) (User, error) {
	return s.repository.CreateUser(ctx, name)
}
