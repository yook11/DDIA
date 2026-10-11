// Command loadgen は、掲示板の投稿処理を並列に呼び出して primary へ書き込み負荷をかける。
// HTTP は通さず、cmd/server と同じ post.Service と Repository をプロセス内で使う。
// 同時に admin 接続で PostgreSQL の待ち・WAL・複製の状態を記録する。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ddia/app/post"
	"ddia/app/thread"
	"ddia/app/user"
	"ddia/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 負荷用の接続はこの名前で pg_stat_activity に現れ、観測はこの名前だけを数える。
const (
	loadAppName    = "ddia-loadgen"
	samplerAppName = "ddia-loadgen-sampler"
)

type config struct {
	appURL    string
	adminURL  string
	mode      string
	out       string
	threads   int
	users     int
	levels    []int
	rates     []int
	pool      int
	warmup    time.Duration
	duration  time.Duration
	drain     time.Duration
	settle    time.Duration
	bodyBytes int
	seed      uint64
	slotGuard int64
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func readConfig(args []string, getenv func(string) string) (config, error) {
	cfg := config{
		appURL:   strings.TrimSpace(getenv("PRIMARY_DATABASE_URL")),
		adminURL: strings.TrimSpace(getenv("ADMIN_DATABASE_URL")),
	}
	var levels, rates string
	var slotGuardMB int64
	fs := flag.NewFlagSet("loadgen", flag.ContinueOnError)
	fs.StringVar(&cfg.mode, "mode", "closed", "closed | open | observe")
	fs.StringVar(&cfg.out, "out", "", "結果を書き出す作成済みディレクトリ")
	fs.IntVar(&cfg.threads, "threads", 1000, "投稿先のスレッド数。1なら全ワーカーが同じスレッドへ書く")
	fs.IntVar(&cfg.users, "users", 100, "投稿者の数")
	fs.StringVar(&levels, "levels", "1,2,4,8,16,32,64", "closed: 並列数の列")
	fs.StringVar(&rates, "rates", "", "open: 毎秒の投稿数の列")
	fs.IntVar(&cfg.pool, "pool", 0, "primaryへの最大接続数。closedでは0なら最大並列数")
	fs.DurationVar(&cfg.warmup, "warmup", 5*time.Second, "各段の計測前の慣らし時間")
	fs.DurationVar(&cfg.duration, "duration", 30*time.Second, "各段の計測時間（observeでは観測時間）")
	fs.DurationVar(&cfg.drain, "drain", 10*time.Second, "open: 送信停止後に完了を待つ上限")
	fs.DurationVar(&cfg.settle, "settle", 60*time.Second, "段の間でレプリカの追いつきを待つ上限")
	fs.IntVar(&cfg.bodyBytes, "body-bytes", 200, "投稿本文のバイト数")
	fs.Uint64Var(&cfg.seed, "seed", 1, "スレッドと投稿者の選択に使う乱数の種")
	fs.Int64Var(&slotGuardMB, "slot-guard-mb", 512, "スロットの残り保持量がこれを下回ったら中断する")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	cfg.slotGuard = slotGuardMB << 20

	if cfg.adminURL == "" {
		return config{}, errors.New("ADMIN_DATABASE_URL is required")
	}
	if cfg.out == "" {
		return config{}, errors.New("-out is required")
	}
	var err error
	switch cfg.mode {
	case "observe":
		return cfg, nil
	case "closed":
		if cfg.levels, err = parseInts(levels); err != nil {
			return config{}, fmt.Errorf("-levels: %w", err)
		}
		if cfg.pool == 0 {
			cfg.pool = slices.Max(cfg.levels)
		}
	case "open":
		if cfg.rates, err = parseInts(rates); err != nil {
			return config{}, fmt.Errorf("-rates: %w", err)
		}
		if cfg.pool <= 0 {
			return config{}, errors.New("-pool is required in open mode")
		}
	default:
		return config{}, fmt.Errorf("unknown -mode %q", cfg.mode)
	}
	if cfg.appURL == "" {
		return config{}, errors.New("PRIMARY_DATABASE_URL is required")
	}
	if cfg.threads < 1 || cfg.users < 1 || cfg.bodyBytes < 1 {
		return config{}, errors.New("-threads, -users and -body-bytes must be positive")
	}
	return cfg, nil
}

func parseInts(s string) ([]int, error) {
	var values []int
	for _, field := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || n < 1 {
			return nil, fmt.Errorf("positive integers separated by commas: %q", s)
		}
		values = append(values, n)
	}
	return values, nil
}

func run(ctx context.Context, args []string, getenv func(string) string) error {
	cfg, err := readConfig(args, getenv)
	if err != nil {
		return err
	}
	label := filepath.Base(cfg.out)

	admin, err := openAdmin(ctx, cfg.adminURL)
	if err != nil {
		return err
	}
	defer admin.Close()
	server, err := readServerInfo(ctx, admin)
	if err != nil {
		return err
	}

	if cfg.mode == "observe" {
		return observe(ctx, cfg, admin, server)
	}

	if limit := server.maxConnections - 10; cfg.pool > limit {
		return fmt.Errorf("-pool %d exceeds max_connections %d minus headroom", cfg.pool, server.maxConnections)
	}
	appURL, err := withLoadParams(cfg.appURL, cfg.pool)
	if err != nil {
		return err
	}
	startupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	primary, err := postgres.OpenPrimary(startupCtx, appURL)
	cancel()
	if err != nil {
		return err
	}
	defer primary.Close()

	router := postgres.NewRouter(primary, nil)
	posts := post.NewService(post.NewRepository(router))
	w, err := prepareWorkload(
		ctx,
		user.NewService(user.NewRepository(primary)),
		thread.NewService(thread.NewRepository(router)),
		label, cfg,
	)
	if err != nil {
		return err
	}
	if err := writeMeta(cfg, label, server, args); err != nil {
		return err
	}

	probe := startStallProbe()
	defer probe.stop()
	runCtx, abort := context.WithCancelCause(ctx)
	defer abort(nil)
	sampler, err := startSampler(ctx, admin, cfg.out, cfg.slotGuard, abort)
	if err != nil {
		return err
	}
	defer sampler.stop()

	report, err := newReport(cfg.out)
	if err != nil {
		return err
	}
	defer report.close()

	values := cfg.levels
	if cfg.mode == "open" {
		values = cfg.rates
	}
	for i, value := range values {
		if i > 0 {
			sampler.waitCaughtUp(runCtx, cfg.settle)
		}
		var lv *level
		if cfg.mode == "closed" {
			lv = runClosed(runCtx, posts, primary, w, value, cfg.seed, cfg.warmup, cfg.duration)
		} else {
			lv = runOpen(runCtx, posts, primary, w, value, cfg.seed, cfg.warmup, cfg.duration, cfg.drain)
		}
		lv.pool = cfg.pool
		if err := report.add(lv, sampler.window(lv.measureAt, lv.stopAt), probe.snapshot()); err != nil {
			return err
		}
		if runCtx.Err() != nil {
			break
		}
	}
	report.printTable()
	if cause := context.Cause(runCtx); cause != nil && !errors.Is(cause, context.Canceled) {
		return fmt.Errorf("run aborted: %w", cause)
	}
	return ctx.Err()
}

// withLoadParams は接続数の上限と観測用の接続名を URL に付ける。既存の指定は上書きする。
func withLoadParams(rawURL string, pool int) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return "", errors.New("PRIMARY_DATABASE_URL must be a postgres:// URL")
	}
	q := u.Query()
	q.Set("pool_max_conns", strconv.Itoa(pool))
	q.Set("application_name", loadAppName)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func openAdmin(ctx context.Context, rawURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(rawURL)
	if err != nil {
		return nil, errors.New("invalid ADMIN_DATABASE_URL")
	}
	cfg.MaxConns = 2
	cfg.ConnConfig.RuntimeParams["application_name"] = samplerAppName
	startupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(startupCtx, cfg)
	if err != nil {
		return nil, errors.New("invalid ADMIN_DATABASE_URL")
	}
	if err := pool.Ping(startupCtx); err != nil {
		pool.Close()
		return nil, errors.New("could not connect to primary with ADMIN_DATABASE_URL")
	}
	return pool, nil
}

type serverInfo struct {
	maxConnections int
	settings       map[string]string
	postsRows      int64
	postsBytes     int64
	postsIndex     int64
}

func readServerInfo(ctx context.Context, admin *pgxpool.Pool) (serverInfo, error) {
	info := serverInfo{settings: map[string]string{}}
	var version string
	if err := admin.QueryRow(ctx, `SELECT version()`).Scan(&version); err != nil {
		return info, fmt.Errorf("read server version: %w", err)
	}
	info.settings["version"] = version

	rows, err := admin.Query(ctx, `
		SELECT name, setting || coalesce(unit, '')
		FROM pg_settings
		WHERE name = ANY($1)
	`, []string{
		"max_connections", "shared_buffers", "synchronous_commit", "synchronous_standby_names",
		"wal_level", "wal_buffers", "wal_writer_delay", "commit_delay", "full_page_writes",
		"max_wal_size", "checkpoint_timeout", "max_slot_wal_keep_size",
	})
	if err != nil {
		return info, fmt.Errorf("read server settings: %w", err)
	}
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return info, fmt.Errorf("scan server settings: %w", err)
		}
		info.settings[name] = value
	}
	if err := rows.Err(); err != nil {
		return info, fmt.Errorf("read server settings: %w", err)
	}
	info.maxConnections, _ = strconv.Atoi(info.settings["max_connections"])

	err = admin.QueryRow(ctx, `
		SELECT reltuples::bigint, pg_table_size(oid), pg_indexes_size(oid)
		FROM pg_class
		WHERE oid = 'board.posts'::regclass
	`).Scan(&info.postsRows, &info.postsBytes, &info.postsIndex)
	if err != nil {
		return info, fmt.Errorf("read board.posts size: %w", err)
	}
	return info, nil
}

func writeMeta(cfg config, label string, server serverInfo, args []string) error {
	meta := map[string]any{
		"run":        label,
		"started_at": time.Now(),
		"args":       args,
		"mode":       cfg.mode,
		"threads":    cfg.threads,
		"users":      cfg.users,
		"levels":     cfg.levels,
		"rates":      cfg.rates,
		"pool":       cfg.pool,
		"warmup":     cfg.warmup.String(),
		"duration":   cfg.duration.String(),
		"drain":      cfg.drain.String(),
		"body_bytes": cfg.bodyBytes,
		"seed":       cfg.seed,
		"host": map[string]any{
			"goos": runtime.GOOS, "goarch": runtime.GOARCH, "num_cpu": runtime.NumCPU(),
			"gomaxprocs": runtime.GOMAXPROCS(0), "go": runtime.Version(),
		},
		"server": server.settings,
		// 行数とサイズは実行前の値。表とインデックスが大きくなると結果が変わる。
		"posts_before": map[string]int64{
			"rows_estimate": server.postsRows, "table_bytes": server.postsBytes, "index_bytes": server.postsIndex,
		},
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cfg.out, "meta.json"), data, 0o644)
}

// observe は負荷をかけずに観測だけを行う。観測そのものが動くことと、平常時の値を確かめる。
func observe(ctx context.Context, cfg config, admin *pgxpool.Pool, server serverInfo) error {
	if err := writeMeta(cfg, filepath.Base(cfg.out), server, nil); err != nil {
		return err
	}
	runCtx, abort := context.WithCancelCause(ctx)
	defer abort(nil)
	sampler, err := startSampler(ctx, admin, cfg.out, cfg.slotGuard, abort)
	if err != nil {
		return err
	}
	from := time.Now()
	select {
	case <-time.After(cfg.duration):
	case <-runCtx.Done():
	}
	to := time.Now()
	sampler.stop()
	fmt.Println(formatServer(sampler.window(from, to)))
	if cause := context.Cause(runCtx); cause != nil && !errors.Is(cause, context.Canceled) {
		return fmt.Errorf("observe aborted: %w", cause)
	}
	return nil
}
