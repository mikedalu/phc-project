// Package db owns the PostgreSQL connection pool.
//
// It exists as its own package (rather than living in main) so that repositories
// can depend on *pgxpool.Pool without importing anything HTTP-related, keeping
// the dependency arrows pointing one way: http -> service -> repo -> db.
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool builds the connection pool for the API.
//
// ═══ THE NEON POOLER TRAP ═══════════════════════════════════════════
//
// The API must use Neon's POOLED endpoint (host contains "-pooler"), which is
// PgBouncer running in transaction mode. PgBouncer in that mode does NOT support
// server-side prepared statements, but pgx uses the extended protocol (and
// therefore prepared statements) by default.
//
// The failure is nasty because it is intermittent: it works in development, then
// under concurrency you get
//
//	ERROR: prepared statement "stmtcache_5f8a..." already exists (SQLSTATE 42P05)
//
// Setting QueryExecModeExec bypasses the statement cache entirely, which is
// exactly what pgx's own documentation recommends when running behind a pooler.
//
// Do NOT use this pool for migrations — goose needs the DIRECT endpoint, because
// transaction pooling cannot support DDL inside a long transaction.
func NewPool(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	// ⚠️ ARITHMETIC THAT MATTERS: maxConns x Cloud Run --max-instances must stay
	// below your Neon plan's connection ceiling. With maxConns = 10 and
	// --max-instances = 5 you need 50 connections available. Raise one without
	// checking the other and you get "too many clients already" under load.
	cfg.MaxConns = maxConns

	// MinConns = 1 keeps one warm connection so the first request after a cold
	// start does not pay a full TCP + TLS + auth handshake to Neon.
	cfg.MinConns = 1

	// Recycle connections well inside Neon's own idle timeout, so we are never
	// holding a connection the server has already decided to close.
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close() // do not leak a half-built pool
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// WithTx runs fn inside a transaction, committing on success and rolling back on
// error or panic.
//
// Every multi-statement write in the service layer goes through this. It removes
// the most common transaction bug in hand-written Go, which is forgetting the
// rollback on an early return path.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// Rollback is a no-op after a successful commit, so this is safe to defer
	// unconditionally. That is what makes the pattern reliable.
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err // already wrapped by the caller for context
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// ─── PostgreSQL error classification ───────────────────────────────
//
// These let the service layer translate database errors into domain errors
// without importing pgconn everywhere. Keeping the SQLSTATE codes in one place
// means "what does 23505 mean" is answered once.

// IsUniqueViolation reports whether err is a unique-constraint violation (23505).
//
// In this project that almost always means one of: a duplicate payment operation
// (which is benign — treat it as already applied), a duplicate patient number
// (recoverable — reassign and retry), or a double-booked bed (a real conflict to
// surface to the user).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// IsForeignKeyViolation reports whether err is a foreign-key violation (23503).
// Typically a stale reference from a client that has not pulled recently.
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// IsCheckViolation reports whether err is a CHECK-constraint violation (23514).
// Usually an out-of-range clinical value that should have been caught by
// validation — worth logging, because it means validation and schema disagree.
func IsCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

// IsNotFound reports whether err is pgx.ErrNoRows.
func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// ConstraintName extracts the violated constraint's name, for logging and for
// returning a useful machine-readable reason to the client. Returns "" if err is
// not a PgError.
func ConstraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}
	return ""
}
