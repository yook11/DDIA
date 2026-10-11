package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 待ちの内訳は0.1秒ごと、累積値と複製の状態は1秒ごとに読む。
const (
	activityInterval  = 100 * time.Millisecond
	activityPerSecond = 10
	caughtUpBytes     = 64 << 10
)

// serverSecond は primary から見た約1秒分の状態。差分の値はこの1秒間の増分。
type serverSecond struct {
	T    time.Time `json:"t"`
	Span float64   `json:"span_s"`
	// 負荷用接続の平均数。CPU は待ちのない active、Idle はクライアント側の処理中か未使用。
	Sessions         map[string]float64 `json:"sessions"`
	WALBytes         int64              `json:"wal_bytes"`
	WALRecords       int64              `json:"wal_records"`
	WALFPI           int64              `json:"wal_fpi"`
	WALBuffersFull   int64              `json:"wal_buffers_full"`
	Commits          int64              `json:"commits"`
	PostsInserted    int64              `json:"posts_inserted"`
	ClientFsyncs     int64              `json:"client_wal_fsyncs"`
	ClientFsyncMs    float64            `json:"client_wal_fsync_ms"`
	WalwriterFsyncs  int64              `json:"walwriter_wal_fsyncs"`
	WalwriterFsyncMs float64            `json:"walwriter_wal_fsync_ms"`
	// primary コンテナの cgroup から読んだCPU。ThrottledMs はCPU上限で止められていた時間。
	CPUCores    float64        `json:"primary_cpu_cores"`
	ThrottledMs float64        `json:"primary_throttled_ms"`
	Replicas    []replicaState `json:"replicas"`
	Slots       []slotState    `json:"slots"`
}

type replicaState struct {
	Name string `json:"name"`
	// LagBytes は primary の書き込み位置と適用位置の差。nil は未接続か未適用。
	LagBytes     *int64   `json:"lag_bytes"`
	ReplayLagSec *float64 `json:"replay_lag_s"`
}

type slotState struct {
	Name        string `json:"name"`
	WALStatus   string `json:"wal_status"`
	SafeWALSize *int64 `json:"safe_wal_size"`
}

// counters は統計ビューの累積値。統計はバックエンドが約1秒ごとにまとめて反映するので、
// 1秒ごとの差分は揺れる。比較には計測窓全体の合計を使う。LSN だけは即時の値。
type counters struct {
	lsn              int64
	walRecords       int64
	walFPI           int64
	walBuffersFull   int64
	commits          int64
	postsInserted    int64
	clientFsyncs     int64
	clientFsyncMs    float64
	walwriterFsyncs  int64
	walwriterFsyncMs float64
	cpuUsageUsec     int64 // cgroup が読めなければ -1
	throttledUsec    int64
}

// sampler は admin 接続で primary を観測し、スロットの残りが少なくなったら実行を中断させる。
type sampler struct {
	pool   *pgxpool.Pool
	guard  int64
	abort  context.CancelCauseFunc
	file   *os.File
	enc    *json.Encoder
	cancel context.CancelFunc
	done   chan struct{}

	mu      sync.Mutex
	seconds []serverSecond
}

func startSampler(
	ctx context.Context,
	pool *pgxpool.Pool,
	outDir string,
	guard int64,
	abort context.CancelCauseFunc,
) (*sampler, error) {
	prev, err := readCounters(ctx, pool)
	if err != nil {
		return nil, err
	}
	file, err := os.Create(filepath.Join(outDir, "server.jsonl"))
	if err != nil {
		return nil, err
	}
	samplerCtx, cancel := context.WithCancel(ctx)
	s := &sampler{
		pool:   pool,
		guard:  guard,
		abort:  abort,
		file:   file,
		enc:    json.NewEncoder(file),
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go s.loop(samplerCtx, prev)
	return s, nil
}

func (s *sampler) stop() {
	s.cancel()
	<-s.done
	_ = s.file.Close()
}

func (s *sampler) loop(ctx context.Context, prev counters) {
	defer close(s.done)
	ticker := time.NewTicker(activityInterval)
	defer ticker.Stop()

	sessions := map[string]int{}
	samples := 0
	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if err := sampleActivity(ctx, s.pool, sessions); err != nil {
			s.fail(ctx, err)
			return
		}
		samples++
		if samples < activityPerSecond {
			continue
		}

		cur, err := readCounters(ctx, s.pool)
		if err != nil {
			s.fail(ctx, err)
			return
		}
		replicas, slots, err := readReplication(ctx, s.pool)
		if err != nil {
			s.fail(ctx, err)
			return
		}
		now := time.Now()
		sec := serverSecond{
			T:                now,
			Span:             now.Sub(last).Seconds(),
			Sessions:         make(map[string]float64, len(sessions)),
			WALBytes:         cur.lsn - prev.lsn,
			WALRecords:       cur.walRecords - prev.walRecords,
			WALFPI:           cur.walFPI - prev.walFPI,
			WALBuffersFull:   cur.walBuffersFull - prev.walBuffersFull,
			Commits:          cur.commits - prev.commits,
			PostsInserted:    cur.postsInserted - prev.postsInserted,
			ClientFsyncs:     cur.clientFsyncs - prev.clientFsyncs,
			ClientFsyncMs:    cur.clientFsyncMs - prev.clientFsyncMs,
			WalwriterFsyncs:  cur.walwriterFsyncs - prev.walwriterFsyncs,
			WalwriterFsyncMs: cur.walwriterFsyncMs - prev.walwriterFsyncMs,
			Replicas:         replicas,
			Slots:            slots,
		}
		for key, count := range sessions {
			sec.Sessions[key] = float64(count) / float64(samples)
		}
		if cur.cpuUsageUsec >= 0 && prev.cpuUsageUsec >= 0 {
			sec.CPUCores = float64(cur.cpuUsageUsec-prev.cpuUsageUsec) / 1e6 / sec.Span
			sec.ThrottledMs = float64(cur.throttledUsec-prev.throttledUsec) / 1e3
		}
		s.mu.Lock()
		s.seconds = append(s.seconds, sec)
		s.mu.Unlock()
		if err := s.enc.Encode(sec); err != nil {
			s.fail(ctx, err)
			return
		}
		if err := checkSlots(slots, s.guard); err != nil {
			s.abort(err)
		}

		prev, last = cur, now
		clear(sessions)
		samples = 0
	}
}

// fail は観測できなくなった実行を止める。スロットを見張れないまま負荷をかけ続けない。
func (s *sampler) fail(ctx context.Context, err error) {
	if ctx.Err() == nil {
		s.abort(fmt.Errorf("server sampling failed: %w", err))
	}
}

func checkSlots(slots []slotState, guard int64) error {
	for _, slot := range slots {
		if slot.WALStatus == "unreserved" || slot.WALStatus == "lost" {
			return fmt.Errorf("slot %s is %s; the replica may need to be rebuilt", slot.Name, slot.WALStatus)
		}
		if slot.SafeWALSize != nil && *slot.SafeWALSize < guard {
			return fmt.Errorf("slot %s has only %d MB of WAL left before it is lost", slot.Name, *slot.SafeWALSize>>20)
		}
	}
	return nil
}

func sampleActivity(ctx context.Context, pool *pgxpool.Pool, sessions map[string]int) error {
	rows, err := pool.Query(ctx, `
		SELECT state, coalesce(wait_event_type || ':' || wait_event, ''), count(*)::int
		FROM pg_stat_activity
		WHERE application_name = $1
		GROUP BY 1, 2
	`, loadAppName)
	if err != nil {
		return fmt.Errorf("read pg_stat_activity: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var state *string
		var wait string
		var count int
		if err := rows.Scan(&state, &wait, &count); err != nil {
			return fmt.Errorf("scan pg_stat_activity: %w", err)
		}
		sessions[sessionKey(state, wait)] += count
	}
	return rows.Err()
}

func sessionKey(state *string, wait string) string {
	switch {
	case state == nil:
		return "Unknown"
	case *state == "active" && wait == "":
		return "CPU"
	case *state == "active":
		return wait
	case *state == "idle":
		return "Idle"
	case *state == "idle in transaction":
		return "IdleInTransaction"
	default:
		return *state
	}
}

func readCounters(ctx context.Context, pool *pgxpool.Pool) (counters, error) {
	var c counters
	err := pool.QueryRow(ctx, `
		SELECT
			(pg_current_wal_insert_lsn() - '0/0'::pg_lsn)::bigint,
			w.wal_records, w.wal_fpi, w.wal_buffers_full,
			(SELECT xact_commit FROM pg_stat_database WHERE datname = current_database()),
			(SELECT n_tup_ins FROM pg_stat_user_tables WHERE relid = 'board.posts'::regclass),
			io.client_fsyncs, io.client_fsync_ms, io.walwriter_fsyncs, io.walwriter_fsync_ms
		FROM pg_stat_wal w,
		LATERAL (
			SELECT
				coalesce(sum(fsyncs) FILTER (WHERE backend_type = 'client backend'), 0)::bigint AS client_fsyncs,
				coalesce(sum(fsync_time) FILTER (WHERE backend_type = 'client backend'), 0)::float8 AS client_fsync_ms,
				coalesce(sum(fsyncs) FILTER (WHERE backend_type = 'walwriter'), 0)::bigint AS walwriter_fsyncs,
				coalesce(sum(fsync_time) FILTER (WHERE backend_type = 'walwriter'), 0)::float8 AS walwriter_fsync_ms
			FROM pg_stat_io
			WHERE object = 'wal'
		) io
	`).Scan(
		&c.lsn, &c.walRecords, &c.walFPI, &c.walBuffersFull, &c.commits, &c.postsInserted,
		&c.clientFsyncs, &c.clientFsyncMs, &c.walwriterFsyncs, &c.walwriterFsyncMs,
	)
	if err != nil {
		return c, fmt.Errorf("read server counters: %w", err)
	}
	c.cpuUsageUsec, c.throttledUsec = readCgroupCPU(ctx, pool)
	return c, nil
}

// readCgroupCPU は primary のコンテナ内の cgroup v2 の cpu.stat を読む。読めなければ -1 を返す。
// pg_read_file はスーパーユーザーの admin 接続でだけ使える。
func readCgroupCPU(ctx context.Context, pool *pgxpool.Pool) (usage, throttled int64) {
	var stat string
	if err := pool.QueryRow(ctx, `SELECT pg_read_file('/sys/fs/cgroup/cpu.stat')`).Scan(&stat); err != nil {
		return -1, -1
	}
	usage, throttled = -1, -1
	for _, line := range strings.Split(stat, "\n") {
		name, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			continue
		}
		switch name {
		case "usage_usec":
			usage = n
		case "throttled_usec":
			throttled = n
		}
	}
	return usage, throttled
}

func readReplication(ctx context.Context, pool *pgxpool.Pool) ([]replicaState, []slotState, error) {
	rows, err := pool.Query(ctx, `
		SELECT application_name,
		       pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn)::bigint,
		       extract(epoch FROM replay_lag)::float8
		FROM pg_stat_replication
		ORDER BY application_name
	`)
	if err != nil {
		return nil, nil, fmt.Errorf("read pg_stat_replication: %w", err)
	}
	replicas := []replicaState{}
	for rows.Next() {
		var r replicaState
		if err := rows.Scan(&r.Name, &r.LagBytes, &r.ReplayLagSec); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("scan pg_stat_replication: %w", err)
		}
		replicas = append(replicas, r)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("read pg_stat_replication: %w", err)
	}

	rows, err = pool.Query(ctx, `
		SELECT slot_name, coalesce(wal_status, ''), safe_wal_size
		FROM pg_replication_slots
		ORDER BY slot_name
	`)
	if err != nil {
		return nil, nil, fmt.Errorf("read pg_replication_slots: %w", err)
	}
	slots := []slotState{}
	for rows.Next() {
		var s slotState
		if err := rows.Scan(&s.Name, &s.WALStatus, &s.SafeWALSize); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("scan pg_replication_slots: %w", err)
		}
		slots = append(slots, s)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("read pg_replication_slots: %w", err)
	}
	return replicas, slots, nil
}

// serverWindow は計測窓の間の観測をまとめた値。
type serverWindow struct {
	Seconds                  float64            `json:"seconds"`
	Sessions                 map[string]float64 `json:"sessions"`
	WALBytesPerSec           float64            `json:"wal_bytes_per_s"`
	WALRecordsPerSec         float64            `json:"wal_records_per_s"`
	WALFPIPerSec             float64            `json:"wal_fpi_per_s"`
	WALBuffersFull           int64              `json:"wal_buffers_full"`
	CommitsPerSec            float64            `json:"commits_per_s"`
	PostsInsertedPerSec      float64            `json:"posts_inserted_per_s"`
	ClientFsyncsPerSec       float64            `json:"client_wal_fsyncs_per_s"`
	ClientFsyncMsAvg         float64            `json:"client_wal_fsync_ms_avg"`
	WalwriterFsyncsPerSec    float64            `json:"walwriter_wal_fsyncs_per_s"`
	PrimaryCPUCores          float64            `json:"primary_cpu_cores"`
	PrimaryThrottledMsPerSec float64            `json:"primary_throttled_ms_per_s"`
	MaxLagBytes              int64              `json:"max_lag_bytes"`
	MaxReplayLagSec          float64            `json:"max_replay_lag_s"`
	// MinSafeWALBytes はスロットが失効するまでに書けるWAL量の最小値。-1は不明。
	MinSafeWALBytes int64 `json:"min_safe_wal_bytes"`
}

// window は (from, to] に終わった観測秒をまとめる。
func (s *sampler) window(from, to time.Time) serverWindow {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := serverWindow{Sessions: map[string]float64{}, MinSafeWALBytes: -1}
	var walBytes, walRecords, walFPI, commits, inserted, clientFsyncs, walwriterFsyncs int64
	var clientFsyncMs, cpuSeconds, throttledMs float64
	for _, sec := range s.seconds {
		if !sec.T.After(from) || sec.T.After(to) {
			continue
		}
		w.Seconds += sec.Span
		for key, value := range sec.Sessions {
			w.Sessions[key] += value * sec.Span
		}
		walBytes += sec.WALBytes
		walRecords += sec.WALRecords
		walFPI += sec.WALFPI
		w.WALBuffersFull += sec.WALBuffersFull
		commits += sec.Commits
		inserted += sec.PostsInserted
		clientFsyncs += sec.ClientFsyncs
		clientFsyncMs += sec.ClientFsyncMs
		walwriterFsyncs += sec.WalwriterFsyncs
		cpuSeconds += sec.CPUCores * sec.Span
		throttledMs += sec.ThrottledMs
		for _, r := range sec.Replicas {
			if r.LagBytes != nil {
				w.MaxLagBytes = max(w.MaxLagBytes, *r.LagBytes)
			}
			if r.ReplayLagSec != nil {
				w.MaxReplayLagSec = max(w.MaxReplayLagSec, *r.ReplayLagSec)
			}
		}
		for _, slot := range sec.Slots {
			if slot.SafeWALSize != nil && (w.MinSafeWALBytes < 0 || *slot.SafeWALSize < w.MinSafeWALBytes) {
				w.MinSafeWALBytes = *slot.SafeWALSize
			}
		}
	}
	if w.Seconds == 0 {
		return w
	}
	for key := range w.Sessions {
		w.Sessions[key] /= w.Seconds
	}
	w.WALBytesPerSec = float64(walBytes) / w.Seconds
	w.WALRecordsPerSec = float64(walRecords) / w.Seconds
	w.WALFPIPerSec = float64(walFPI) / w.Seconds
	w.CommitsPerSec = float64(commits) / w.Seconds
	w.PostsInsertedPerSec = float64(inserted) / w.Seconds
	w.ClientFsyncsPerSec = float64(clientFsyncs) / w.Seconds
	w.WalwriterFsyncsPerSec = float64(walwriterFsyncs) / w.Seconds
	w.PrimaryCPUCores = cpuSeconds / w.Seconds
	w.PrimaryThrottledMsPerSec = throttledMs / w.Seconds
	if clientFsyncs > 0 {
		w.ClientFsyncMsAvg = clientFsyncMs / float64(clientFsyncs)
	}
	return w
}

// waitCaughtUp は、呼び出し後の観測で全レプリカの遅れが小さくなるまで待つ。
// 前の段の未適用WALが次の段の計測に混ざらないようにする。
func (s *sampler) waitCaughtUp(ctx context.Context, limit time.Duration) {
	since := time.Now()
	deadline := since.Add(limit)
	for ctx.Err() == nil {
		if s.caughtUpSince(since) {
			return
		}
		if time.Now().After(deadline) {
			log.Printf("replicas did not catch up within %s; continuing", limit)
			return
		}
		select {
		case <-ctx.Done():
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (s *sampler) caughtUpSince(since time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.seconds) == 0 {
		return false
	}
	last := s.seconds[len(s.seconds)-1]
	if !last.T.After(since) {
		return false
	}
	for _, r := range last.Replicas {
		if r.LagBytes == nil || *r.LagBytes > caughtUpBytes {
			return false
		}
	}
	return true
}
