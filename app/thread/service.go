package thread

import (
	"context"

	"ddia/app/user"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) CreateThread(ctx context.Context, authorID user.ID, title string) (Thread, error) {
	return s.repository.CreateThread(ctx, authorID, title)
}
