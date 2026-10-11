package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"

	"ddia/app/thread"
	"ddia/app/user"
)

// workload は書き込み先の候補。実行ごとに新しいユーザーとスレッドを作り、既存データには書かない。
type workload struct {
	threads []thread.ID
	users   []user.ID
	body    string
}

func prepareWorkload(
	ctx context.Context,
	users *user.Service,
	threads *thread.Service,
	label string,
	cfg config,
) (*workload, error) {
	w := &workload{body: strings.Repeat("x", cfg.bodyBytes)}
	for i := range cfg.users {
		created, err := users.CreateUser(ctx, fmt.Sprintf("load-%s-%d", label, i))
		if err != nil {
			return nil, fmt.Errorf("prepare users: %w", err)
		}
		w.users = append(w.users, created.ID)
	}
	for i := range cfg.threads {
		created, err := threads.CreateThread(ctx, w.users[i%len(w.users)], fmt.Sprintf("load %s #%d", label, i))
		if err != nil {
			return nil, fmt.Errorf("prepare threads: %w", err)
		}
		w.threads = append(w.threads, created.Thread.ID)
	}
	return w, nil
}

// pick は一様に投稿先と投稿者を選ぶ。threadsが1つなら全員が同じスレッドへ書く。
func (w *workload) pick(rng *rand.Rand) (thread.ID, user.ID) {
	return w.threads[rng.IntN(len(w.threads))], w.users[rng.IntN(len(w.users))]
}
