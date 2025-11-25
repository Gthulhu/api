package app

import (
	"context"
	"os"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/rest"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/labstack/echo/v4"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

func NewRestApp(configName string, configDirPath string) (*fx.App, error) {
	handlerModule, err := HandlerModule(configName, configDirPath)
	if err != nil {
		return nil, err
	}

	app := fx.New(
		handlerModule,
		fx.Invoke(StartRestApp),
	)
	return app, nil
}

func StartRestApp(lc fx.Lifecycle, cfg config.ServerConfig, handler *rest.Handler) error {
	engine := echo.New()
	handler.SetupRoutes(engine)

	// TODO: setup middleware, logging, etc.

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			serverHost := cfg.Host
			if serverHost == "" {
				serverHost = ":8080"
			}
			go func() {
				logger.Logger(ctx).Info().Msgf("starting rest server on port %s", serverHost)
				if err := engine.Start(serverHost); err != nil {
					logger.Logger(ctx).Fatal().Msgf("start rest server fail on port %s", serverHost)
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

func getConfigInfo(cmd *cobra.Command) (string, string) {
	configName := "manager_config"
	configDirPath := ""
	if cmd != nil {
		configNameFlag, err := cmd.Flags().GetString("config-name")
		if err == nil && configNameFlag != "" {
			configName = configNameFlag
		}
		configPathFlag, err := cmd.Flags().GetString("config-dir")
		if err == nil && configPathFlag != "" {
			configDirPath = configPathFlag
		}
	}
	if envConfigName := os.Getenv("MANAGER_CONFIG_NAME"); envConfigName != "" {
		configName = envConfigName
	}
	if envConfigPath := os.Getenv("MANAGER_CONFIG_DIR_PATH"); envConfigPath != "" {
		configDirPath = envConfigPath
	}
	return configName, configDirPath
}
