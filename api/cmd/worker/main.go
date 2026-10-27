// Command worker runs the scheduler and the background job consumers.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	_ "time/tzdata" // IANA time zones for minimal container images

	"github.com/joho/godotenv"

	"github.com/dev-fritz/newsp/api/internal/bootstrap"
	"github.com/dev-fritz/newsp/api/internal/config"
	"github.com/dev-fritz/newsp/api/internal/database"
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
	logger := bootstrap.NewLogger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	container, err := bootstrap.NewContainer(cfg, logger, db, bootstrap.Options{})
	if err != nil {
		return err
	}
	runner, scheduler := bootstrap.NewWorker(container)
	logger.Info("worker started", "kinds", runner.Kinds())

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); scheduler.Run(ctx) }()
	go func() { defer wg.Done(); runner.Run(ctx) }()
	wg.Wait() // runner returns after in-flight jobs finish

	logger.Info("worker stopped")
	return nil
}
