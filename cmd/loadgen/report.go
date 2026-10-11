package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// clientSecond は1段の中の約1秒分。完了時刻で区切る。
type clientSecond struct {
	Mode       string  `json:"mode"`
	Value      int     `json:"value"`
	Second     int     `json:"second"`
	Phase      string  `json:"phase"` // warmup | measure | tail
	OK         int     `json:"ok"`
	Errors     int     `json:"errors"`
	Unfinished int     `json:"unfinished"`
	P50Ms      float64 `json:"p50_ms"`
	P99Ms      float64 `json:"p99_ms"`
	MaxMs      float64 `json:"max_ms"`
	// InFlight はこの秒の終わりに送信済みで未完了の投稿数。open では待ち行列の長さになる。
	InFlight   int     `json:"in_flight"`
	PoolWaits  int64   `json:"pool_waits"`
	PoolWaitMs float64 `json:"pool_wait_ms"`
	// StallMs はこの秒に終わった、負荷生成器自身が止まっていた時間。PostgreSQL の待ちではない。
	StallMs float64 `json:"loadgen_stall_ms"`
}

// summary は1段の計測窓のまとめ。
type summary struct {
	Mode             string   `json:"mode"`
	Value            int      `json:"value"`
	Pool             int      `json:"pool"`
	ThroughputPerSec float64  `json:"throughput_per_s"`
	MeanMs           float64  `json:"mean_ms"`
	P50Ms            float64  `json:"p50_ms"`
	P95Ms            float64  `json:"p95_ms"`
	P99Ms            float64  `json:"p99_ms"`
	MaxMs            float64  `json:"max_ms"`
	Errors           int      `json:"errors"`
	Unfinished       int      `json:"unfinished"`
	ErrorSamples     []string `json:"error_samples,omitempty"`
	// LittleConcurrency はスループット×平均レイテンシ。closed では並列数とほぼ一致するはず。
	LittleConcurrency float64 `json:"little_concurrency"`
	// PoolWaitMsPerOp は投稿1回あたりの接続待ち時間。レイテンシのうちクライアント側で待った分。
	PoolWaitMsPerOp float64      `json:"pool_wait_ms_per_op"`
	Stalls          int          `json:"loadgen_stalls"`
	StallMs         float64      `json:"loadgen_stall_ms"`
	Server          serverWindow `json:"server"`
}

type report struct {
	clientFile  *os.File
	clientEnc   *json.Encoder
	summaryFile *os.File
	summaryEnc  *json.Encoder
	rows        []summary
}

func newReport(dir string) (*report, error) {
	clientFile, err := os.Create(filepath.Join(dir, "client.jsonl"))
	if err != nil {
		return nil, err
	}
	summaryFile, err := os.Create(filepath.Join(dir, "summary.jsonl"))
	if err != nil {
		_ = clientFile.Close()
		return nil, err
	}
	return &report{
		clientFile:  clientFile,
		clientEnc:   json.NewEncoder(clientFile),
		summaryFile: summaryFile,
		summaryEnc:  json.NewEncoder(summaryFile),
	}, nil
}

func (r *report) close() {
	_ = r.clientFile.Close()
	_ = r.summaryFile.Close()
}

func (r *report) add(lv *level, srv serverWindow, stalls []stall) error {
	for _, sec := range clientSeconds(lv, stalls) {
		if err := r.clientEnc.Encode(sec); err != nil {
			return err
		}
	}
	sum := summarize(lv, srv)
	for _, st := range stalls {
		if !st.end.Before(lv.measureAt) && st.end.Before(lv.stopAt) {
			sum.Stalls++
			sum.StallMs += ms(st.duration)
		}
	}
	r.rows = append(r.rows, sum)
	fmt.Println(formatSummary(sum))
	return r.summaryEnc.Encode(sum)
}

func isCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func clientSeconds(lv *level, stalls []stall) []clientSecond {
	last := lv.stopAt
	for _, o := range lv.ops {
		if o.end.After(last) {
			last = o.end
		}
	}
	n := int(last.Sub(lv.start)/time.Second) + 1
	latencies := make([][]time.Duration, n)
	seconds := make([]clientSecond, n)
	for i := range seconds {
		bucketStart := lv.start.Add(time.Duration(i) * time.Second)
		phase := "measure"
		switch {
		case bucketStart.Before(lv.measureAt):
			phase = "warmup"
		case !bucketStart.Before(lv.stopAt):
			phase = "tail"
		}
		seconds[i] = clientSecond{Mode: lv.mode, Value: lv.value, Second: i, Phase: phase}
	}
	for _, o := range lv.ops {
		i := int(o.end.Sub(lv.start) / time.Second)
		switch {
		case o.err == nil:
			seconds[i].OK++
			latencies[i] = append(latencies[i], o.end.Sub(o.sched))
		case isCanceled(o.err):
			seconds[i].Unfinished++
		default:
			seconds[i].Errors++
		}
	}
	for i := range seconds {
		boundary := lv.start.Add(time.Duration(i+1) * time.Second)
		for _, o := range lv.ops {
			if o.sched.Before(boundary) && o.end.After(boundary) {
				seconds[i].InFlight++
			}
		}
		if len(latencies[i]) > 0 {
			slices.Sort(latencies[i])
			seconds[i].P50Ms = ms(percentile(latencies[i], 0.50))
			seconds[i].P99Ms = ms(percentile(latencies[i], 0.99))
			seconds[i].MaxMs = ms(latencies[i][len(latencies[i])-1])
		}
	}
	for _, st := range stalls {
		if i := int(st.end.Sub(lv.start) / time.Second); !st.end.Before(lv.start) && i < n {
			seconds[i].StallMs += ms(st.duration)
		}
	}
	for k := 1; k < len(lv.poolStats) && k-1 < n; k++ {
		prev, cur := lv.poolStats[k-1], lv.poolStats[k]
		seconds[k-1].PoolWaits = cur.empty - prev.empty
		seconds[k-1].PoolWaitMs = ms(cur.emptyWait - prev.emptyWait)
	}
	return seconds
}

func summarize(lv *level, srv serverWindow) summary {
	measure := lv.stopAt.Sub(lv.measureAt)
	inWindow := func(t time.Time) bool { return !t.Before(lv.measureAt) && t.Before(lv.stopAt) }

	sum := summary{Mode: lv.mode, Value: lv.value, Pool: lv.pool, Server: srv}
	var latencies []time.Duration
	completed := 0
	for _, o := range lv.ops {
		if o.err == nil && inWindow(o.end) {
			completed++
		}
		// closed は窓の中で終わった投稿、open は窓の中で送る予定だった投稿のレイテンシを見る。
		counted := inWindow(o.end)
		if lv.mode == "open" {
			counted = inWindow(o.sched)
		}
		if !counted {
			continue
		}
		switch {
		case o.err == nil:
			latencies = append(latencies, o.end.Sub(o.sched))
		case isCanceled(o.err):
			sum.Unfinished++
		default:
			sum.Errors++
			if msg := o.err.Error(); len(sum.ErrorSamples) < 3 && !slices.Contains(sum.ErrorSamples, msg) {
				sum.ErrorSamples = append(sum.ErrorSamples, msg)
			}
		}
	}
	sum.ThroughputPerSec = float64(completed) / measure.Seconds()
	if len(latencies) > 0 {
		slices.Sort(latencies)
		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		mean := total / time.Duration(len(latencies))
		sum.MeanMs = ms(mean)
		sum.P50Ms = ms(percentile(latencies, 0.50))
		sum.P95Ms = ms(percentile(latencies, 0.95))
		sum.P99Ms = ms(percentile(latencies, 0.99))
		sum.MaxMs = ms(latencies[len(latencies)-1])
		sum.LittleConcurrency = sum.ThroughputPerSec * mean.Seconds()
	}

	var first, last *poolSample
	for i := range lv.poolStats {
		p := &lv.poolStats[i]
		if first == nil && !p.t.Before(lv.measureAt) {
			first = p
		}
		if !p.t.After(lv.stopAt) {
			last = p
		}
	}
	if first != nil && last != nil && last.t.After(first.t) {
		finished := 0
		for _, o := range lv.ops {
			if !o.end.Before(first.t) && o.end.Before(last.t) {
				finished++
			}
		}
		if finished > 0 {
			sum.PoolWaitMsPerOp = ms(last.emptyWait-first.emptyWait) / float64(finished)
		}
	}
	return sum
}

// percentile は昇順に並んだ値から最近接順位で取り出す。
func percentile(sorted []time.Duration, p float64) time.Duration {
	i := int(float64(len(sorted))*p+0.5) - 1
	return sorted[min(max(i, 0), len(sorted)-1)]
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func formatSummary(s summary) string {
	return fmt.Sprintf(
		"%-6s %5d | %8.1f/s mean %6.2fms p50 %6.2f p99 %7.2f max %7.2f | err %d unfinished %d | L=%.1f poolwait %.2fms | stall %d×, %.0fms\n       %s",
		s.Mode, s.Value, s.ThroughputPerSec, s.MeanMs, s.P50Ms, s.P99Ms, s.MaxMs,
		s.Errors, s.Unfinished, s.LittleConcurrency, s.PoolWaitMsPerOp, s.Stalls, s.StallMs,
		formatServer(s.Server),
	)
}

func formatServer(w serverWindow) string {
	return fmt.Sprintf(
		"srv %.0f commit/s %.0f insert/s | cpu %.2f cores, throttled %.0fms/s | fsync %.0f/s (%.1f insert/fsync, %.2fms) walwriter %.0f/s | wal %.2fMB/s fpi %.0f/s bufFull %d | lag ≤%.2fMB %.2fs | %s",
		w.CommitsPerSec, w.PostsInsertedPerSec, w.PrimaryCPUCores, w.PrimaryThrottledMsPerSec,
		w.ClientFsyncsPerSec, insertsPerFsync(w), w.ClientFsyncMsAvg, w.WalwriterFsyncsPerSec,
		w.WALBytesPerSec/(1<<20), w.WALFPIPerSec, w.WALBuffersFull,
		float64(w.MaxLagBytes)/(1<<20), w.MaxReplayLagSec,
		formatSessions(w.Sessions),
	)
}

// insertsPerFsync は WAL の fsync 1回でまとめてディスクに書けた投稿の数。
// LSN を読む SELECT もコミットに数えられるが fsync しないので、コミット数ではなく投稿数で割る。
func insertsPerFsync(w serverWindow) float64 {
	if w.ClientFsyncsPerSec == 0 {
		return 0
	}
	return w.PostsInsertedPerSec / w.ClientFsyncsPerSec
}

// formatSessions は平均セッション数の多い順に並べる。
func formatSessions(sessions map[string]float64) string {
	keys := make([]string, 0, len(sessions))
	for key, value := range sessions {
		if value >= 0.05 {
			keys = append(keys, key)
		}
	}
	slices.SortFunc(keys, func(a, b string) int {
		switch {
		case sessions[a] > sessions[b]:
			return -1
		case sessions[a] < sessions[b]:
			return 1
		default:
			return strings.Compare(a, b)
		}
	})
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s %.1f", key, sessions[key]))
	}
	if len(parts) == 0 {
		return "no load sessions"
	}
	return strings.Join(parts, ", ")
}

func (r *report) printTable() {
	fmt.Println()
	fmt.Printf("%-6s %5s %9s %8s %8s %8s %6s %6s %7s %8s %9s %9s %8s %8s %s\n",
		"mode", "value", "ok/s", "mean_ms", "p99_ms", "L", "err", "cpu", "thr_ms", "fsync/s", "ins/fsync", "wal_MB/s", "lag_MB", "stall_ms", "top sessions")
	for _, s := range r.rows {
		fmt.Printf("%-6s %5d %9.1f %8.2f %8.2f %8.1f %6d %6.2f %7.0f %8.0f %9.1f %9.2f %8.2f %8.0f %s\n",
			s.Mode, s.Value, s.ThroughputPerSec, s.MeanMs, s.P99Ms, s.LittleConcurrency,
			s.Errors+s.Unfinished, s.Server.PrimaryCPUCores, s.Server.PrimaryThrottledMsPerSec, s.Server.ClientFsyncsPerSec, insertsPerFsync(s.Server),
			s.Server.WALBytesPerSec/(1<<20), float64(s.Server.MaxLagBytes)/(1<<20), s.StallMs,
			formatSessions(s.Server.Sessions))
	}
}
