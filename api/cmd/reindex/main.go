// Command reindex enqueues a full embedding reindex. The worker runs it:
// items are reset and re-embedded, and ideas and user topics are re-embedded.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/dev-fritz/newsp/api/internal/config"
	"github.com/dev-fritz/newsp/api/internal/database"
	"github.com/dev-fritz/newsp/api/internal/repositories"
	"github.com/dev-fritz/newsp/api/internal/services"
	"github.com/dev-fritz/newsp/api/internal/workers"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	queue := workers.NewQueue(repositories.NewJobRepository(db), func() time.Time { return time.Now().UTC() })
	id, err := queue.Enqueue(ctx, services.JobReindexEmbeddings, struct{}{}, workers.EnqueueOptions{DedupeKey: services.JobReindexEmbeddings})
	if err != nil {
		return fmt.Errorf("enqueue %s: %w", services.JobReindexEmbeddings, err)
	}
	fmt.Println(id)
	return nil
}
