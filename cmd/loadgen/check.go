package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// numberCheck は実行後の不変条件の検査結果。
// レス番号はスレッド内で1から欠番・重複なく続くこと（重複は一意制約で起きえないので、
// 最小が1で最大が件数と等しいことを確かめる）と、完了を返した投稿がすべてDBにあること。
type numberCheck struct {
	Threads      int            `json:"threads"`
	Acknowledged int            `json:"acknowledged"`
	Stored       int64          `json:"stored"`
	Broken       []brokenThread `json:"broken_threads"`
}

type brokenThread struct {
	ThreadID  int64 `json:"thread_id"`
	Posts     int64 `json:"posts"`
	MinNumber int64 `json:"min_number"`
	MaxNumber int64 `json:"max_number"`
}

func checkPostNumbers(ctx context.Context, pool *pgxpool.Pool, w *workload, acknowledged int, outDir string) error {
	ids := make([]int64, 0, len(w.threads))
	for _, id := range w.threads {
		n, err := strconv.ParseInt(string(id), 10, 64)
		if err != nil {
			return fmt.Errorf("check post numbers: thread ID %q: %w", id, err)
		}
		ids = append(ids, n)
	}

	result := numberCheck{Threads: len(ids), Acknowledged: acknowledged, Broken: []brokenThread{}}
	rows, err := pool.Query(ctx, `
		SELECT thread_id, count(*), min(post_number), max(post_number)
		FROM board.posts
		WHERE thread_id = ANY($1)
		GROUP BY thread_id
	`, ids)
	if err != nil {
		return fmt.Errorf("check post numbers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t brokenThread
		if err := rows.Scan(&t.ThreadID, &t.Posts, &t.MinNumber, &t.MaxNumber); err != nil {
			return fmt.Errorf("check post numbers: %w", err)
		}
		result.Stored += t.Posts
		if t.MinNumber != 1 || t.MaxNumber != t.Posts {
			result.Broken = append(result.Broken, t)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("check post numbers: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "check.json"), data, 0o644); err != nil {
		return err
	}

	// 完了を返さなかった投稿（打ち切りなど）がコミットされていることはあるので、
	// 保存数が完了数より多いのは異常ではない。少ない場合は完了を返した投稿が消えている。
	status := "ok"
	if len(result.Broken) > 0 || result.Stored < int64(result.Acknowledged) {
		status = "VIOLATED"
	}
	fmt.Printf("\ncheck %s: %d threads, acknowledged %d, stored %d, threads with gaps %d\n",
		status, result.Threads, result.Acknowledged, result.Stored, len(result.Broken))
	return nil
}
