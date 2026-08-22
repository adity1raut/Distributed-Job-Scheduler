package repository

import (
	"context"

	"github.com/adity1raut/job-scheduler/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RetryPolicyRepository struct {
	pool *pgxpool.Pool
}

func NewRetryPolicyRepository(pool *pgxpool.Pool) *RetryPolicyRepository {
	return &RetryPolicyRepository{pool: pool}
}

func (r *RetryPolicyRepository) Create(ctx context.Context, p *models.RetryPolicy) (*models.RetryPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO retry_policies (org_id, name, strategy, base_delay_ms, max_delay_ms, max_attempts, multiplier)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, org_id, name, strategy, base_delay_ms, max_delay_ms, max_attempts, multiplier, created_at`,
		p.OrgID, p.Name, p.Strategy, p.BaseDelayMS, p.MaxDelayMS, p.MaxAttempts, p.Multiplier)
	if err != nil {
		return nil, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.RetryPolicy])
}

func (r *RetryPolicyRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*models.RetryPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, strategy, base_delay_ms, max_delay_ms, max_attempts, multiplier, created_at
		 FROM retry_policies WHERE id = $1 AND org_id = $2`, id, orgID)
	if err != nil {
		return nil, err
	}
	p, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.RetryPolicy])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

// GetByIDInternal looks up a policy by ID with no org scoping — used only
// by internal callers (the execution engine) that already resolved the
// policy through a job or queue they've verified, not by request handlers.
func (r *RetryPolicyRepository) GetByIDInternal(ctx context.Context, id uuid.UUID) (*models.RetryPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, strategy, base_delay_ms, max_delay_ms, max_attempts, multiplier, created_at
		 FROM retry_policies WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	p, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.RetryPolicy])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *RetryPolicyRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]models.RetryPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, strategy, base_delay_ms, max_delay_ms, max_attempts, multiplier, created_at
		 FROM retry_policies WHERE org_id = $1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.RetryPolicy])
}

// EnsureDefault returns the org's "default" retry policy, creating a sane
// exponential-backoff one on first use so queue creation never needs a
// client to supply a policy up front.
func (r *RetryPolicyRepository) EnsureDefault(ctx context.Context, orgID uuid.UUID) (*models.RetryPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, strategy, base_delay_ms, max_delay_ms, max_attempts, multiplier, created_at
		 FROM retry_policies WHERE org_id = $1 AND name = 'default'`, orgID)
	if err != nil {
		return nil, err
	}
	p, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[models.RetryPolicy])
	if err == nil {
		return p, nil
	}
	if err != pgx.ErrNoRows {
		return nil, err
	}

	return r.Create(ctx, &models.RetryPolicy{
		OrgID:       orgID,
		Name:        "default",
		Strategy:    models.RetryExponential,
		BaseDelayMS: 5000,
		MaxDelayMS:  60000,
		MaxAttempts: 5,
		Multiplier:  2,
	})
}
