package app

import (
	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/repository"
	"github.com/Gthulhu/api/manager/rest"
	"github.com/Gthulhu/api/manager/service"
	"go.uber.org/fx"
)

func ConfigModule(configName string, configPath string) (fx.Option, error) {
	cfg, err := config.InitManagerConfig(configName, configPath)
	if err != nil {
		return nil, err
	}

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
	), nil
}

// RepoModule creates an Fx module that provides the repository layer, return repository.Repository
func RepoModule(configName string, configPath string) (fx.Option, error) {
	configModule, err := ConfigModule(configName, configPath)
	if err != nil {
		return nil, err
	}

	return fx.Options(
		configModule,
		fx.Provide(repository.NewRepository),
	), nil
}

// ServiceModule creates an Fx module that provides the service layer, return domain.Service
func ServiceModule(configName string, configPath string) (fx.Option, error) {
	repoModule, err := RepoModule(configName, configPath)
	if err != nil {
		return nil, err
	}

	return fx.Options(
		repoModule,
		fx.Provide(service.NewService),
	), nil
}

// HandlerModule creates an Fx module that provides the REST handler, return *rest.Handler
func HandlerModule(configName string, configPath string) (fx.Option, error) {
	serviceModule, err := ServiceModule(configName, configPath)
	if err != nil {
		return nil, err
	}

	return fx.Options(
		serviceModule,
		fx.Provide(rest.NewHandler),
	), nil
}
