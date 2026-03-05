package app

import (
	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/client"
	"github.com/Gthulhu/api/manager/domain"
	k8sadapter "github.com/Gthulhu/api/manager/k8s_adapter"
	"github.com/Gthulhu/api/manager/repository"
	"github.com/Gthulhu/api/manager/rest"
	"github.com/Gthulhu/api/manager/service"
	"github.com/Gthulhu/api/pkg/container"
	"go.uber.org/fx"
)

// ConfigModule creates an Fx module that provides configuration structs
func ConfigModule(cfg config.ManageConfig) (fx.Option, error) {
	return fx.Options(
		fx.Provide(func() config.ManageConfig {
			return cfg
		}),
		fx.Provide(func(managerCfg config.ManageConfig) config.MongoDBConfig {
			return managerCfg.MongoDB
		}),
		fx.Provide(func(managerCfg config.ManageConfig) config.ServerConfig {
			return managerCfg.Server
		}),
		fx.Provide(func(managerCfg config.ManageConfig) config.KeyConfig {
			return managerCfg.Key
		}),
		fx.Provide(func(managerCfg config.ManageConfig) config.AccountConfig {
			return managerCfg.Account
		}),
		fx.Provide(func(managerCfg config.ManageConfig) config.K8SConfig {
			return managerCfg.K8S
		}),
		fx.Provide(func(managerCfg config.ManageConfig) config.MTLSConfig {
			return managerCfg.MTLS
		}),
	), nil
}

// AdapterModule creates an Fx module that provides the K8S adapter and Decision Maker client
func AdapterModule() (fx.Option, error) {
	return fx.Options(
		fx.Provide(func(k8sConfig config.K8SConfig) (domain.K8SAdapter, error) {
			return k8sadapter.NewAdapter(k8sadapter.Options{
				KubeConfigPath: k8sConfig.KubeConfigPath,
				InCluster:      k8sConfig.IsInCluster,
			})
		}),
		fx.Provide(client.NewDecisionMakerClient),
	), nil
}

// RepoModule creates an Fx module that provides the repository layer, return repository.Repository
func RepoModule(cfg config.ManageConfig) (fx.Option, error) {
	configModule, err := ConfigModule(cfg)
	if err != nil {
		return nil, err
	}

	return fx.Options(
		configModule,
		fx.Provide(repository.NewRepository),
	), nil
}

// ServiceModule creates an Fx module that provides the service layer, return domain.Service
func ServiceModule(adapterModule, repoModule fx.Option) (fx.Option, error) {
	return fx.Options(
		adapterModule,
		repoModule,
		fx.Provide(service.NewService),
	), nil
}

// HandlerModule creates an Fx module that provides the REST handler, return *rest.Handler
func HandlerModule(serviceModule fx.Option) (fx.Option, error) {
	return fx.Options(
		serviceModule,
		fx.Provide(rest.NewHandler),
	), nil
}

// TestRepoModule creates an Fx module that provides the repository layer for testing, return repository.Repository
func TestRepoModule(cfg config.ManageConfig, containerBuilder *container.ContainerBuilder) (fx.Option, error) {
	configModule, err := ConfigModule(cfg)
	if err != nil {
		return nil, err
	}
	_, err = container.RunMongoContainer(containerBuilder, "api_test_mongo", container.MongoContainerConnection{
		Username: cfg.MongoDB.User,
		Password: string(cfg.MongoDB.Password),
		Database: cfg.MongoDB.Database,
		Port:     cfg.MongoDB.Port,
		Host:     cfg.MongoDB.Host,
	})
	if err != nil {
		return nil, err
	}
	return fx.Options(
		configModule,
		fx.Provide(repository.NewRepository),
	), nil
}
