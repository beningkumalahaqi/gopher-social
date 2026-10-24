// Package database owns the PostgreSQL connection pool and low-level helpers.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Open parses the URL, builds a bounded pool and verifies connectivity. A
// server that starts without a database should fail immediately, not on the
// first request.
func Open(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = 1
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// IsUniqueViolation reports whether err is a PostgreSQL unique constraint
// violation (SQLSTATE 23505).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return isPgCode(err, &pgErr) && pgErr.Code == "23505"
}

// IsForeignKeyViolation reports whether err is a foreign key violation
// (SQLSTATE 23503), typically caused by a referenced row disappearing.
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return isPgCode(err, &pgErr) && pgErr.Code == "23503"
}

func isPgCode(err error, target **pgconn.PgError) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	*target = pgErr
	return true
}
