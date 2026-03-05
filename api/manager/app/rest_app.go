package app

import (
	"context"
	"time"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/migration"
	"github.com/Gthulhu/api/manager/rest"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

const (
	reconcileInterval    = 30 * time.Second
	reconcileInitialWait = 5 * time.Second
)

func NewRestApp(configName string, configDirPath string) (*fx.App, error) {
	cfg, err := config.InitManagerConfig(configName, configDirPath)
	if err != nil {
		return nil, err
	}

	repoModule, err := RepoModule(cfg)
	if err != nil {
		return nil, err
	}

	adapterModule, err := AdapterModule()
	if err != nil {
		return nil, err
	}

	serviceModule, err := ServiceModule(adapterModule, repoModule)
	if err != nil {
		return nil, err
	}

	handlerModule, err := HandlerModule(serviceModule)
	if err != nil {
		return nil, err
	}

	app := fx.New(
		handlerModule,
		fx.Invoke(migration.RunMongoMigration),
		fx.Invoke(StartRestApp),
		fx.Invoke(StartIntentReconciler),
	)
	return app, nil
}

func StartRestApp(lc fx.Lifecycle, cfg config.ServerConfig, handler *rest.Handler) error {
	engine := echo.New()
	handler.SetupRoutes(engine)
	rest.RegisterFrontend(engine)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			serverHost := cfg.Host
			if serverHost == "" {
				serverHost = ":8080"
			}
			go func() {
				logger.Logger(ctx).Info().Msgf("starting rest server on port %s", serverHost)
				if err := engine.Start(serverHost); err != nil {
					logger.Logger(ctx).Fatal().Err(err).Msgf("start rest server fail on port %s", serverHost)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Logger(ctx).Info().Msg("shutting down rest server")
			return engine.Shutdown(ctx)
		},
	})

	return nil
}

// StartIntentReconciler starts a background goroutine that periodically
// reconciles scheduling intents. This handles:
// - Manager restart: re-sends all intents from DB to DM pods
// - Decision Maker restart: detects Merkle root mismatch and re-sends intents
// - Pod restart: detects stale intents and creates new ones for replacement pods
func StartIntentReconciler(lc fx.Lifecycle, svc domain.Service) error {
	stopCh := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				bgCtx := context.Background()
				logger.Logger(bgCtx).Info().Msgf("intent reconciler starting, initial wait %s, interval %s", reconcileInitialWait, reconcileInterval)

				// Wait briefly for DM pods to be ready before first reconciliation
				select {
				case <-time.After(reconcileInitialWait):
				case <-stopCh:
					return
				}

				// Run initial reconciliation on startup
				logger.Logger(bgCtx).Info().Msg("running initial intent reconciliation")
				if err := svc.ReconcileIntents(bgCtx); err != nil {
					logger.Logger(bgCtx).Warn().Err(err).Msg("initial intent reconciliation failed")
				}

				ticker := time.NewTicker(reconcileInterval)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						if err := svc.ReconcileIntents(bgCtx); err != nil {
							logger.Logger(bgCtx).Warn().Err(err).Msg("periodic intent reconciliation failed")
						}
					case <-stopCh:
						logger.Logger(bgCtx).Info().Msg("intent reconciler stopped")
						return
					}
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			close(stopCh)
			return nil
		},
	})

	return nil
}
