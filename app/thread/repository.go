package thread

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"ddia/app/user"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidAuthorID = errors.New("invalid thread author ID")
	ErrAuthorNotFound  = errors.New("thread author not found")
)

type Repository interface {
	CreateThread(ctx context.Context, authorID user.ID, title string) (Thread, error)
}

type repository struct {
	primary *pgxpool.Pool
}

func NewRepository(primary *pgxpool.Pool) Repository {
	return &repository{primary: primary}
}

func (r *repository) CreateThread(ctx context.Context, authorID user.ID, title string) (Thread, error) {
	authorNumber, err := strconv.ParseInt(string(authorID), 10, 64)
	if err != nil || authorNumber <= 0 {
		return Thread{}, ErrInvalidAuthorID
	}

	var id int64
	var storedAuthorID int64
	var storedTitle string
	err = r.primary.QueryRow(ctx, `
		INSERT INTO board.threads (author_id, title)
		VALUES ($1, $2)
		RETURNING id, author_id, title
	`, authorNumber, title).Scan(&id, &storedAuthorID, &storedTitle)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) &&
			postgresError.Code == "23503" &&
			postgresError.ConstraintName == "threads_author_id_fkey" {
			return Thread{}, ErrAuthorNotFound
		}
		return Thread{}, fmt.Errorf("create thread: %w", err)
	}

	return Thread{
		ID:       ID(strconv.FormatInt(id, 10)),
		Title:    storedTitle,
		AuthorID: user.ID(strconv.FormatInt(storedAuthorID, 10)),
	}, nil
}

var _ Repository = (*repository)(nil)
