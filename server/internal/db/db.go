package db

// Pool wraps the Postgres connection pool.
// TODO: replace with *pgxpool.Pool once github.com/jackc/pgx/v5 is added (go get + go mod tidy).
type Pool struct{}

// New opens a Postgres connection pool using the given DSN.
func New(dsn string) (*Pool, error) {
	// TODO: return pgxpool.New(context.Background(), dsn)
	return &Pool{}, nil
}

func (p *Pool) Close() {
	// TODO: p.pool.Close()
}
