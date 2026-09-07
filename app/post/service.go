package post

import (
	"context"

	"ddia/app/thread"
	"ddia/app/user"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) CreatePost(
	ctx context.Context,
	threadID thread.ID,
	authorID user.ID,
	body string,
) (Post, error) {
	return s.repository.CreatePost(ctx, threadID, authorID, body)
}
