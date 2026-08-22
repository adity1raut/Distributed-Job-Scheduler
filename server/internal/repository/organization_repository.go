package repository

import (
	"context"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrganizationRepository struct {
	pool *pgxpool.Pool
}

func NewOrganizationRepository(pool *pgxpool.Pool) *OrganizationRepository {
	return &OrganizationRepository{pool: pool}
}

func (r *OrganizationRepository) Create(ctx context.Context, name string) (*models.Organization, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO organizations (name) VALUES ($1) RETURNING id, name, created_at`, name)
	if err != nil {
		return nil, err
	}
	org, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Organization])
	if err != nil {
		return nil, err
	}
	return org, nil
}

func (r *OrganizationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, created_at FROM organizations WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	org, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Organization])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return org, nil
}
