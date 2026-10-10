package post

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"

	"ddia/app/thread"
	"ddia/app/user"
	"ddia/postgres"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInvalidThreadID = errors.New("invalid post thread ID")
	ErrInvalidAuthorID = errors.New("invalid post author ID")
	ErrThreadNotFound  = errors.New("post thread not found")
	ErrAuthorNotFound  = errors.New("post author not found")
)

type Repository interface {
	CreatePost(ctx context.Context, threadID thread.ID, authorID user.ID, body string) (WriteResult, error)
}

type repository struct {
	router *postgres.Router
}

func NewRepository(router *postgres.Router) Repository {
	return &repository{router: router}
}

func (r *repository) CreatePost(
	ctx context.Context,
	threadID thread.ID,
	authorID user.ID,
	body string,
) (WriteResult, error) {
	threadNumber, err := strconv.ParseInt(string(threadID), 10, 64)
	if err != nil || threadNumber <= 0 {
		return WriteResult{}, ErrInvalidThreadID
	}
	authorNumber, err := strconv.ParseInt(string(authorID), 10, 64)
	if err != nil || authorNumber <= 0 {
		return WriteResult{}, ErrInvalidAuthorID
	}

	var id int64
	var storedThreadID int64
	var storedAuthorID int64
	var storedBody string

	writePool := r.router.WritePool()
	err = writePool.QueryRow(ctx, `
		INSERT INTO board.posts (thread_id, author_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, thread_id, author_id, body
	`, threadNumber, authorNumber, body).Scan(&id, &storedThreadID, &storedAuthorID, &storedBody)
	if err != nil {
		return WriteResult{}, mapCreatePostError(err)
	}

	position, err := postgres.CurrentPosition(ctx, writePool)
	if err != nil {
		log.Printf("create post %d: position unavailable: %v", id, err)
		position = ""
	}

	return WriteResult{
		Post: Post{
			ID:       ID(strconv.FormatInt(id, 10)),
			ThreadID: thread.ID(strconv.FormatInt(storedThreadID, 10)),
			AuthorID: user.ID(strconv.FormatInt(storedAuthorID, 10)),
			Body:     storedBody,
			ReplyTo:  nil,
		},
		Position: position,
	}, nil
}

func mapCreatePostError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23503" {
		switch postgresError.ConstraintName {
		case "posts_thread_id_fkey":
			return ErrThreadNotFound
		case "posts_author_id_fkey":
			return ErrAuthorNotFound
		}
	}
	return fmt.Errorf("create post: %w", err)
}

var _ Repository = (*repository)(nil)
