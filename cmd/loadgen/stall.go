package main

import (
	"slices"
	"sync"
	"time"
)

// 負荷生成器自身が止まっていた時間を記録する。20msごとに起き、予定より100ms以上遅れたら停止とみなす。
// Compose ネットワーク内で動かすと、Docker VM 全体の停止や自分のCPU制限がここに現れる。
// PostgreSQL の中の待ちではタイマーは遅れないので、レイテンシの山がどちらによるものかを区別できる。
const (
	stallTick      = 20 * time.Millisecond
	stallThreshold = 100 * time.Millisecond
)

type stall struct {
	end      time.Time
	duration time.Duration
}

type stallProbe struct {
	done     chan struct{}
	finished chan struct{}

	mu     sync.Mutex
	stalls []stall
}

func startStallProbe() *stallProbe {
	p := &stallProbe{done: make(chan struct{}), finished: make(chan struct{})}
	go func() {
		defer close(p.finished)
		ticker := time.NewTicker(stallTick)
		defer ticker.Stop()
		last := time.Now()
		for {
			select {
			case <-p.done:
				return
			case <-ticker.C:
				now := time.Now()
				if gap := now.Sub(last); gap >= stallThreshold {
					p.mu.Lock()
					p.stalls = append(p.stalls, stall{end: now, duration: gap})
					p.mu.Unlock()
				}
				last = now
			}
		}
	}()
	return p
}

func (p *stallProbe) stop() {
	close(p.done)
	<-p.finished
}

func (p *stallProbe) snapshot() []stall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.stalls)
}
