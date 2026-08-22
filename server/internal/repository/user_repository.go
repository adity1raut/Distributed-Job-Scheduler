package repository

import (
	"context"
	"errors"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, orgID uuid.UUID, email, passwordHash string, role models.Role) (*models.User, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO users (org_id, email, password_hash, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, org_id, email, password_hash, role, created_at`,
		orgID, email, passwordHash, role)
	if err != nil {
		return nil, err
	}
	user, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.User])
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrConflict
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, email, password_hash, role, created_at FROM users WHERE email = $1`, email)
	if err != nil {
		return nil, err
	}
	user, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.User])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, email, password_hash, role, created_at FROM users WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	user, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.User])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return user, nil
}
