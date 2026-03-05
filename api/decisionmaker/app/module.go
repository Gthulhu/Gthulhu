package app

import (
	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/decisionmaker/rest"
	"github.com/Gthulhu/api/decisionmaker/service"
	"go.uber.org/fx"
)

// ConfigModule creates an Fx module that provides configuration structs
func ConfigModule(cfg config.DecisionMakerConfig) (fx.Option, error) {
	return fx.Options(
		fx.Provide(func() config.DecisionMakerConfig {
			return cfg
		}),
		fx.Provide(func(dmCfg config.DecisionMakerConfig) config.ServerConfig {
			return dmCfg.Server
		}),
		fx.Provide(func(dmCfg config.DecisionMakerConfig) config.TokenConfig {
			return dmCfg.Token
		}),
		fx.Provide(func(dmCfg config.DecisionMakerConfig) config.MTLSConfig {
			return dmCfg.MTLS
		}),
	), nil
}

func ServiceModule() (fx.Option, error) {
	return fx.Options(
		fx.Provide(service.NewService),
	), nil
}

// HandlerModule creates an Fx module that provides the REST handler, return *rest.Handler
func HandlerModule(opt fx.Option) (fx.Option, error) {
	return fx.Options(
		opt,
		fx.Provide(rest.NewHandler),
	), nil
}
