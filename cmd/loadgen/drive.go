package main

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"ddia/app/post"

	"github.com/jackc/pgx/v5/pgxpool"
)

// op は投稿1回の記録。レイテンシは end - sched。
type op struct {
	// closedでは送信時刻、openでは予定時刻。openでは接続待ちもレイテンシに含める。
	sched time.Time
	end   time.Time
	err   error
}

// level は並列数または送信レートを1つに固定した1段分の実行。
type level struct {
	mode      string
	value     int // closed: 並列数、open: 毎秒の投稿数
	pool      int
	start     time.Time
	measureAt time.Time
	stopAt    time.Time
	ops       []op
	poolStats []poolSample
}

type poolSample struct {
	t         time.Time
	acquires  int64
	empty     int64
	emptyWait time.Duration
}

func newLevel(mode string, value int, warmup, measure time.Duration) *level {
	start := time.Now()
	return &level{
		mode:      mode,
		value:     value,
		start:     start,
		measureAt: start.Add(warmup),
		stopAt:    start.Add(warmup + measure),
	}
}

// runClosed は n 個のワーカーで、各自が前の投稿の完了を待ってから次を送る。
// stopAt 以降は新しく送らず、送信済みの投稿は完了まで待つ。
func runClosed(
	ctx context.Context,
	posts *post.Service,
	pool *pgxpool.Pool,
	w *workload,
	n int,
	seed uint64,
	warmup, measure time.Duration,
) *level {
	lv := newLevel("closed", n, warmup, measure)
	stopPool := samplePool(pool, &lv.poolStats)
	perWorker := make([][]op, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			rng := rand.New(rand.NewPCG(seed, uint64(i)))
			for ctx.Err() == nil && time.Now().Before(lv.stopAt) {
				threadID, authorID := w.pick(rng)
				start := time.Now()
				_, err := posts.CreatePost(ctx, threadID, authorID, w.body)
				perWorker[i] = append(perWorker[i], op{sched: start, end: time.Now(), err: err})
			}
		})
	}
	wg.Wait()
	stopPool()
	for _, ops := range perWorker {
		lv.ops = append(lv.ops, ops...)
	}
	return lv
}

// runOpen は完了を待たずに一定間隔で投稿を送る。容量を超えた分は接続待ちとして溜まる。
// stopAt 以降は送らず、drain の間に終わらなかった投稿は打ち切る。
func runOpen(
	ctx context.Context,
	posts *post.Service,
	pool *pgxpool.Pool,
	w *workload,
	rate int,
	seed uint64,
	warmup, measure, drain time.Duration,
) *level {
	lv := newLevel("open", rate, warmup, measure)
	stopPool := samplePool(pool, &lv.poolStats)
	opsCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	rng := rand.New(rand.NewPCG(seed, 0))
	interval := time.Second / time.Duration(rate)
	var mu sync.Mutex
	var wg sync.WaitGroup
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for k := 0; ctx.Err() == nil; {
		now := time.Now()
		for {
			sched := lv.start.Add(time.Duration(k) * interval)
			if !sched.Before(lv.stopAt) || sched.After(now) {
				break
			}
			threadID, authorID := w.pick(rng)
			wg.Go(func() {
				_, err := posts.CreatePost(opsCtx, threadID, authorID, w.body)
				end := time.Now()
				mu.Lock()
				lv.ops = append(lv.ops, op{sched: sched, end: end, err: err})
				mu.Unlock()
			})
			k++
		}
		if !lv.start.Add(time.Duration(k) * interval).Before(lv.stopAt) {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
		}
	}

	timer := time.AfterFunc(time.Until(lv.stopAt.Add(drain)), cancel)
	wg.Wait()
	timer.Stop()
	stopPool()
	return lv
}

// samplePool は接続プールの累積値を1秒ごとに記録する。差分で接続待ちを求める。
func samplePool(pool *pgxpool.Pool, out *[]poolSample) (stop func()) {
	record := func() {
		s := pool.Stat()
		*out = append(*out, poolSample{
			t:         time.Now(),
			acquires:  s.AcquireCount(),
			empty:     s.EmptyAcquireCount(),
			emptyWait: s.EmptyAcquireWaitTime(),
		})
	}
	record()
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				record()
			case <-done:
				record()
				return
			}
		}
	}()
	return func() {
		close(done)
		<-finished
	}
}
