package scheduler

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func waitForShutdown(ctx context.Context) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	select {
	case sig := <-sigCh:
		slog.Info("received signal, shutting down", "signal", sig)
	case <-ctx.Done():
	}
	slog.Info("Gthulhu exit")
	return nil
}
