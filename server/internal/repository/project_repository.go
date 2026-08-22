package repository

import (
	"context"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectRepository struct {
	pool *pgxpool.Pool
}

func NewProjectRepository(pool *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{pool: pool}
}

func (r *ProjectRepository) Create(ctx context.Context, orgID, ownerID uuid.UUID, name string) (*models.Project, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO projects (org_id, owner_id, name) VALUES ($1, $2, $3)
		 RETURNING id, org_id, owner_id, name, created_at`,
		orgID, ownerID, name)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Project])
}

func (r *ProjectRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*models.Project, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, owner_id, name, created_at FROM projects WHERE id = $1 AND org_id = $2`,
		id, orgID)
	if err != nil {
		return nil, err
	}
	p, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.Project])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *ProjectRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]models.Project, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, owner_id, name, created_at FROM projects WHERE org_id = $1 ORDER BY created_at DESC`,
		orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.Project])
}

func (r *ProjectRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1 AND org_id = $2`, id, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
