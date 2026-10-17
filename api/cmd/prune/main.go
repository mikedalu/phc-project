// Command prune trims the replication change log.
//
// A separate binary rather than a goroutine inside the API: pruning is an
// operational task with a schedule, it should be runnable by hand when a clinic
// notices the table growing, and it must not compete with request handling for
// connections.
//
// Intended deployment: a Cloud Run Job on a timer. Locally: `make prune`.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikedalu/phc-records/api/internal/config"
	"github.com/mikedalu/phc-records/api/internal/repo"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("ping", "error", err)
		os.Exit(1)
	}

	deleted, err := repo.NewSync().PruneChangeLog(ctx, pool)
	if err != nil {
		logger.Error("prune", "error", err)
		os.Exit(1)
	}

	if len(deleted) == 0 {
		logger.Info("nothing to prune")
		return
	}

	var total int64
	for table, count := range deleted {
		logger.Info("pruned", "table", table, "rows", count)
		total += count
	}
	fmt.Printf("pruned %d change-log rows\n", total)
}
