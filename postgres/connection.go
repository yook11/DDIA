package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

func OpenPrimary(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	primary, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		// Driver errors can contain the connection string, including credentials.
		return nil, errors.New("invalid PRIMARY_DATABASE_URL")
	}
	if err := primary.Ping(ctx); err != nil {
		primary.Close()
		return nil, errors.New("could not connect to primary within the startup deadline")
	}

	var writablePrimary bool
	err = primary.QueryRow(ctx, `
		SELECT NOT pg_is_in_recovery()
		   AND current_setting('transaction_read_only') = 'off'
	`).Scan(&writablePrimary)
	if err != nil {
		primary.Close()
		return nil, errors.New("could not verify primary role and write access")
	}
	if !writablePrimary {
		primary.Close()
		return nil, errors.New("PRIMARY_DATABASE_URL must point to a writable primary")
	}
	return primary, nil
}

func OpenReplica(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	replica, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, errors.New("invalid REPLICA_DATABASE_URL")
	}
	if err := replica.Ping(ctx); err != nil {
		replica.Close()
		return nil, errors.New("could not connect to replica within the startup deadline")
	}

	var isReplica bool
	err = replica.QueryRow(ctx, `SELECT pg_is_in_recovery()`).Scan(&isReplica)
	if err != nil {
		replica.Close()
		return nil, errors.New("could not verify replica role")
	}
	if !isReplica {
		replica.Close()
		return nil, errors.New("REPLICA_DATABASE_URL must point to a replica")
	}
	return replica, nil
}
