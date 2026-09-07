package user

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateUser(ctx context.Context, name string) (User, error)
}

type userRepository struct {
	primary *pgxpool.Pool
}

func NewRepository(primary *pgxpool.Pool) Repository {
	return &userRepository{primary: primary}
}

func (r *userRepository) CreateUser(ctx context.Context, name string) (User, error) {
	var id int64
	var storedName string
	err := r.primary.QueryRow(ctx, `
		INSERT INTO board.users (name)
		VALUES ($1)
		RETURNING id, name
	`, name).Scan(&id, &storedName)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return User{ID: ID(strconv.FormatInt(id, 10)), Name: storedName}, nil
}

var _ Repository = (*userRepository)(nil)
