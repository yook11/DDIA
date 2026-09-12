package postgres

import (
	"ddia/app/readpolicy"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Router struct {
	primary *pgxpool.Pool
	replica *pgxpool.Pool
}

func NewRouter(primary *pgxpool.Pool, replica *pgxpool.Pool) *Router {
	return &Router{primary: primary, replica: replica}
}

func (r *Router) WritePool() *pgxpool.Pool {
	return r.primary
}

func (r *Router) ReadPool(policy readpolicy.Policy) *pgxpool.Pool {
	switch policy.(type) {
	case readpolicy.Eventual:
		return r.replica
	case readpolicy.ReadYourWrites:
		return r.primary
	default:
		return r.primary
	}
}
