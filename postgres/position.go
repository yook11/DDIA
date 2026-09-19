package postgres

import (
	"context"
	"fmt"

	"ddia/app/readpolicy"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CurrentPosition は primary の現在の WAL 位置を返す。
// コミット完了後に呼ぶこと。返る値は必ずそのコミット以降の位置になる。
func CurrentPosition(ctx context.Context, pool *pgxpool.Pool) (readpolicy.Position, error) {
	var lsn string
	err := pool.QueryRow(ctx, "SELECT pg_current_wal_insert_lsn()::text").Scan(&lsn)
	if err != nil {
		return "", fmt.Errorf("failed to get current LSN: %w", err)
	}
	return readpolicy.Position(lsn), nil
}
