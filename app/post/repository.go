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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInvalidThreadID = errors.New("invalid post thread ID")
	ErrInvalidAuthorID = errors.New("invalid post author ID")
	ErrThreadNotFound  = errors.New("post thread not found")
	ErrAuthorNotFound  = errors.New("post author not found")
	// ErrPostNumberConflict は、同じレス番号を他の投稿が先に取り、やり直しても取れなかったことを表す。
	ErrPostNumberConflict = errors.New("post number conflict")
)

// Numbering はレス番号の採番方式。
type Numbering int

const (
	// Pessimistic はスレッドの行の post_count を進めて採番する。
	// 同じスレッドへの投稿は、この行のロックでコミットまで直列になる。
	Pessimistic Numbering = iota
	// Optimistic はロックを取らずに「スレッド内の最大の番号 + 1」で書き、
	// 一意制約 (thread_id, post_number) に違反したら最大値を読み直してやり直す。
	// post_count は更新しない。更新すると、それ自体が人気スレッドのホットな行に戻るため。
	// 同じスレッドで Pessimistic と混ぜると、古い post_count から振った番号がぶつかる。
	Optimistic
)

// maxOptimisticAttempts は、楽観的な採番で一意制約違反をやり直す上限。
const maxOptimisticAttempts = 100

func ParseNumbering(s string) (Numbering, error) {
	switch s {
	case "pessimistic", "":
		return Pessimistic, nil
	case "optimistic":
		return Optimistic, nil
	default:
		return 0, fmt.Errorf("unknown post numbering %q (use pessimistic or optimistic)", s)
	}
}

func (n Numbering) String() string {
	switch n {
	case Pessimistic:
		return "pessimistic"
	case Optimistic:
		return "optimistic"
	default:
		return fmt.Sprintf("Numbering(%d)", int(n))
	}
}

type Repository interface {
	CreatePost(ctx context.Context, threadID thread.ID, authorID user.ID, body string) (WriteResult, error)
}

type repository struct {
	router    *postgres.Router
	numbering Numbering
}

func NewRepository(router *postgres.Router, numbering Numbering) Repository {
	return &repository{router: router, numbering: numbering}
}

// storedPost は INSERT ... RETURNING で受け取った、DB上の型のままの投稿。
type storedPost struct {
	id       int64
	threadID int64
	authorID int64
	body     string
	number   int
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

	var stored storedPost
	switch r.numbering {
	case Pessimistic:
		stored, err = r.insertPessimistic(ctx, threadNumber, authorNumber, body)
	case Optimistic:
		stored, err = r.insertOptimistic(ctx, threadNumber, authorNumber, body)
	default:
		err = fmt.Errorf("create post: unknown numbering %v", r.numbering)
	}
	if err != nil {
		return WriteResult{}, err
	}

	position, err := postgres.CurrentPosition(ctx, r.router.WritePool())
	if err != nil {
		log.Printf("create post %d: position unavailable: %v", stored.id, err)
		position = ""
	}

	return WriteResult{
		Post: Post{
			ID:       ID(strconv.FormatInt(stored.id, 10)),
			ThreadID: thread.ID(strconv.FormatInt(stored.threadID, 10)),
			AuthorID: user.ID(strconv.FormatInt(stored.authorID, 10)),
			Body:     stored.body,
			Number:   stored.number,
			ReplyTo:  nil,
		},
		Position: position,
	}, nil
}

// insertPessimistic はスレッドの行で採番する。
// 採番とINSERTを1文（1トランザクション）にし、ロックを持ったままクライアントと往復しない。
func (r *repository) insertPessimistic(
	ctx context.Context,
	threadNumber, authorNumber int64,
	body string,
) (storedPost, error) {
	stored, err := scanStoredPost(r.router.WritePool().QueryRow(ctx, `
		WITH numbered AS (
			UPDATE board.threads
			SET post_count = post_count + 1
			WHERE id = $1
			RETURNING id, post_count
		)
		INSERT INTO board.posts (thread_id, author_id, body, post_number)
		SELECT numbered.id, $2, $3, numbered.post_count FROM numbered
		RETURNING id, thread_id, author_id, body, post_number
	`, threadNumber, authorNumber, body))
	if errors.Is(err, pgx.ErrNoRows) {
		return storedPost{}, ErrThreadNotFound
	}
	if err != nil {
		return storedPost{}, mapCreatePostError(err)
	}
	return stored, nil
}

// insertOptimistic はロックを取らずに最大の番号 + 1 で書く。
// READ COMMITTED では1文ごとに新しい時点を読むので、やり直すたびに他の投稿が取った番号の続きになる。
// 負けた投稿は待たずにすぐやり直す。待ち時間を入れた版との比較は計測の後で行う。
func (r *repository) insertOptimistic(
	ctx context.Context,
	threadNumber, authorNumber int64,
	body string,
) (storedPost, error) {
	writePool := r.router.WritePool()
	for attempt := 1; ; attempt++ {
		// スレッドがなければ最大値は0になり、1番のINSERTが外部キー違反になる。
		stored, err := scanStoredPost(writePool.QueryRow(ctx, `
			INSERT INTO board.posts (thread_id, author_id, body, post_number)
			SELECT $1, $2, $3, coalesce(max(post_number), 0) + 1
			FROM board.posts
			WHERE thread_id = $1
			RETURNING id, thread_id, author_id, body, post_number
		`, threadNumber, authorNumber, body))
		if err == nil {
			return stored, nil
		}

		err = mapCreatePostError(err)
		if !errors.Is(err, ErrPostNumberConflict) {
			return storedPost{}, err
		}
		if attempt == maxOptimisticAttempts {
			return storedPost{}, fmt.Errorf("%w after %d attempts", ErrPostNumberConflict, attempt)
		}
		if err := ctx.Err(); err != nil {
			return storedPost{}, err
		}
	}
}

func scanStoredPost(row pgx.Row) (storedPost, error) {
	var p storedPost
	err := row.Scan(&p.id, &p.threadID, &p.authorID, &p.body, &p.number)
	return p, err
}

func mapCreatePostError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch {
		case postgresError.Code == "23503" && postgresError.ConstraintName == "posts_thread_id_fkey":
			return ErrThreadNotFound
		case postgresError.Code == "23503" && postgresError.ConstraintName == "posts_author_id_fkey":
			return ErrAuthorNotFound
		case postgresError.Code == "23505" && postgresError.ConstraintName == "posts_thread_id_post_number_key":
			return ErrPostNumberConflict
		}
	}
	return fmt.Errorf("create post: %w", err)
}

var _ Repository = (*repository)(nil)
