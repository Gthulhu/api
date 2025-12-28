package app

import (
	"context"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/decisionmaker/rest"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

func NewRestApp(configName string, configDirPath string) (*fx.App, error) {
	cfg, err := config.InitDMConfig(configName, configDirPath)
	if err != nil {
		return nil, err
	}
	cfgModule, err := ConfigModule(cfg)
	if err != nil {
		return nil, err
	}
	svcModule, err := ServiceModule()
	handlerModule, err := HandlerModule(fx.Options(cfgModule, svcModule))
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
				serverHost = ":8082"
			}
			go func() {
				logger.Logger(ctx).Info().Msgf("starting dm server on port %s", serverHost)
				if err := engine.Start(serverHost); err != nil {
					logger.Logger(ctx).Fatal().Err(err).Msgf("start rest server fail on port %s", serverHost)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Logger(ctx).Info().Msg("shutting down dm server")
			return engine.Shutdown(ctx)
		},
	})

	return nil
}
