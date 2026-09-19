package postgres

import (
	"context"
	"fmt"

	"ddia/app/readpolicy"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Router struct {
	primary *pgxpool.Pool
	replica []*pgxpool.Pool
}

func NewRouter(
	primary *pgxpool.Pool,
	replica []*pgxpool.Pool,
) *Router {
	return &Router{primary: primary, replica: replica}
}

func (r *Router) WritePool() *pgxpool.Pool {
	return r.primary
}

func (r *Router) ReadPool(
	ctx context.Context,
	policy readpolicy.Policy,
) (*pgxpool.Pool, error) {
	switch p := policy.(type) {
	case readpolicy.Eventual:
		if len(r.replica) > 0 {
			return r.replica[0], nil
		}
		return r.primary, nil

	case readpolicy.ReadYourWrites:
		if p.WritePosition == "" {
			return r.primary, nil
		}
		for _, replica := range r.replica {
			caughtUp, err := r.replicaCaughtUp(ctx, replica, p.WritePosition)
			if err != nil {
				return nil, err
			}
			if caughtUp {
				return replica, nil
			}
		}
		return r.primary, nil

	default:
		return r.primary, nil
	}
}

func (r *Router) replicaCaughtUp(
	ctx context.Context,
	replica *pgxpool.Pool,
	WritePosition readpolicy.Position,
) (bool, error) {

	var caughtUp bool

	err := replica.QueryRow(
		ctx,
		`SELECT COALESCE(
			pg_last_wal_replay_lsn() >= $1::pg_lsn,
			false
		)`,
		WritePosition,
	).Scan(&caughtUp)
	if err != nil {
		return false, fmt.Errorf("failed to check if replica caught up: %w", err)
	}
	return caughtUp, nil
}
