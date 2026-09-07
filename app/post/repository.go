package post

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"ddia/app/thread"
	"ddia/app/user"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidThreadID = errors.New("invalid post thread ID")
	ErrInvalidAuthorID = errors.New("invalid post author ID")
	ErrThreadNotFound  = errors.New("post thread not found")
	ErrAuthorNotFound  = errors.New("post author not found")
)

type Repository interface {
	CreatePost(ctx context.Context, threadID thread.ID, authorID user.ID, body string) (Post, error)
}

type repository struct {
	primary *pgxpool.Pool
}

func NewRepository(primary *pgxpool.Pool) Repository {
	return &repository{primary: primary}
}

func (r *repository) CreatePost(
	ctx context.Context,
	threadID thread.ID,
	authorID user.ID,
	body string,
) (Post, error) {
	threadNumber, err := strconv.ParseInt(string(threadID), 10, 64)
	if err != nil || threadNumber <= 0 {
		return Post{}, ErrInvalidThreadID
	}
	authorNumber, err := strconv.ParseInt(string(authorID), 10, 64)
	if err != nil || authorNumber <= 0 {
		return Post{}, ErrInvalidAuthorID
	}

	var id int64
	var storedThreadID int64
	var storedAuthorID int64
	var storedBody string
	err = r.primary.QueryRow(ctx, `
		INSERT INTO board.posts (thread_id, author_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, thread_id, author_id, body
	`, threadNumber, authorNumber, body).Scan(&id, &storedThreadID, &storedAuthorID, &storedBody)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23503" {
			switch postgresError.ConstraintName {
			case "posts_thread_id_fkey":
				return Post{}, ErrThreadNotFound
			case "posts_author_id_fkey":
				return Post{}, ErrAuthorNotFound
			}
		}
		return Post{}, fmt.Errorf("create post: %w", err)
	}

	return Post{
		ID:       ID(strconv.FormatInt(id, 10)),
		ThreadID: thread.ID(strconv.FormatInt(storedThreadID, 10)),
		AuthorID: user.ID(strconv.FormatInt(storedAuthorID, 10)),
		Body:     storedBody,
		ReplyTo:  nil,
	}, nil
}

var _ Repository = (*repository)(nil)
