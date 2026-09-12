package thread

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"ddia/app/readpolicy"
	"ddia/app/user"
	"ddia/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidAuthorID = errors.New("invalid thread author ID")
	ErrAuthorNotFound  = errors.New("thread author not found")
	ErrInvalidThreadID = errors.New("invalid thread ID")
	ErrThreadNotFound  = errors.New("thread not found")
)

type Repository interface {
	CreateThread(ctx context.Context, authorID user.ID, title string) (Thread, error)
	GetThread(ctx context.Context, threadID ID, policy readpolicy.Policy) (View, error)
}

type repository struct {
	router *postgres.Router
}

func NewRepository(router *postgres.Router) Repository {
	return &repository{router: router}
}

func (r *repository) CreateThread(ctx context.Context, authorID user.ID, title string) (Thread, error) {
	authorNumber, err := strconv.ParseInt(string(authorID), 10, 64)
	if err != nil || authorNumber <= 0 {
		return Thread{}, ErrInvalidAuthorID
	}

	var id int64
	var storedAuthorID int64
	var storedTitle string
	writePool := r.router.WritePool()
	err = writePool.QueryRow(ctx, `
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

func (r *repository) GetThread(
	ctx context.Context,
	threadID ID,
	policy readpolicy.Policy,
) (View, error) {
	threadNumber, err := strconv.ParseInt(string(threadID), 10, 64)
	if err != nil || threadNumber <= 0 {
		return View{}, ErrInvalidThreadID
	}

	readPool := r.router.ReadPool(policy)
	thread, err := fetchThread(ctx, readPool, threadNumber)
	if err != nil {
		return View{}, err
	}

	posts, err := fetchThreadPosts(ctx, readPool, threadNumber)
	if err != nil {
		return View{}, err
	}

	return View{Thread: thread, Posts: posts}, nil
}

func fetchThread(ctx context.Context, readPool *pgxpool.Pool, threadNumber int64) (Thread, error) {
	var storedID int64
	var storedAuthorID int64
	var storedTitle string
	err := readPool.QueryRow(ctx, `
		SELECT id, author_id, title
		FROM board.threads
		WHERE id = $1
	`, threadNumber).Scan(&storedID, &storedAuthorID, &storedTitle)
	if errors.Is(err, pgx.ErrNoRows) {
		return Thread{}, ErrThreadNotFound
	}
	if err != nil {
		return Thread{}, fmt.Errorf("get thread: %w", err)
	}

	return Thread{
		ID:       ID(strconv.FormatInt(storedID, 10)),
		Title:    storedTitle,
		AuthorID: user.ID(strconv.FormatInt(storedAuthorID, 10)),
	}, nil
}

func fetchThreadPosts(ctx context.Context, readPool *pgxpool.Pool, threadNumber int64) ([]ThreadPost, error) {
	rows, err := readPool.Query(ctx, `
		SELECT id, thread_id, author_id, body, reply_to
		FROM board.posts
		WHERE thread_id = $1
		ORDER BY id
	`, threadNumber)
	if err != nil {
		return nil, fmt.Errorf("get thread posts: %w", err)
	}
	defer rows.Close()

	posts := make([]ThreadPost, 0)
	for rows.Next() {
		var postID int64
		var postThreadID int64
		var postAuthorID int64
		var body string
		var replyTo pgtype.Int8
		if err := rows.Scan(&postID, &postThreadID, &postAuthorID, &body, &replyTo); err != nil {
			return nil, fmt.Errorf("scan thread post: %w", err)
		}

		post := ThreadPost{
			ID:       PostID(strconv.FormatInt(postID, 10)),
			ThreadID: ID(strconv.FormatInt(postThreadID, 10)),
			AuthorID: user.ID(strconv.FormatInt(postAuthorID, 10)),
			Body:     body,
		}
		if replyTo.Valid {
			replyToID := PostID(strconv.FormatInt(replyTo.Int64, 10))
			post.ReplyTo = &replyToID
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read thread posts: %w", err)
	}

	return posts, nil
}

var _ Repository = (*repository)(nil)
