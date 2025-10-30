package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Gthulhu/api/adapter/kubernetes"
	"github.com/Gthulhu/api/cache"
	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/rest"
	"github.com/Gthulhu/api/service"
	"github.com/Gthulhu/api/util"
	"github.com/gorilla/mux"
)

func main() {
	// Parse command line options
	cmdOptions := ParseCommandLineOptions()
	PrintCommandLineOptions(cmdOptions)

	logger := util.GetLogger()

	// Load configuration
	cfg, err := config.LoadConfig(cmdOptions.ConfigPath)
	if err != nil {
		logger.Error("Failed to load configuration, exit", util.LogErrAttr(err))
		return
	}
	jwtRsaKey, err := config.InitJWTRsaKey(cfg.JWT)
	if err != nil {
		logger.Error("Failed to init jwt rsa key, exit", util.LogErrAttr(err))
		return
	}

	k8sAdapter, err := kubernetes.NewK8SAdapter(kubernetes.Options{
		KubeConfigPath: cmdOptions.KubeConfigPath,
		InCluster:      cmdOptions.InCluster,
	})
	if err != nil {
		logger.Error("Failed to init k8s adapter, exit", util.LogErrAttr(err))
		return
	}

	strategyCache := cache.NewStrategyCache()
	stopPodWatcher, err := cache.StartPodWatcher(strategyCache, k8sAdapter.GetClient())
	if err != nil {
		logger.Error("Failed to start pod watcher, exit", util.LogErrAttr(err))
		return
	}

	svc, err := service.NewService(context.Background(), service.Params{
		K8sAdapter:    k8sAdapter,
		JWTPrivateKey: jwtRsaKey,
		Config:        cfg,
		StrategyCache: strategyCache,
	})
	if err != nil {
		logger.Error("Failed to create service, exit", util.LogErrAttr(err))
		return
	}

	hdl := rest.NewHandler(rest.Params{
		Service:       svc,
		JWTPrivateKey: jwtRsaKey,
		Config:        cfg,
	})

	// If port is specified in command line, override the port in config file
	port := cfg.Server.Port
	if cmdOptions.Port != "" {
		port = cmdOptions.Port
		logger.Info("Overriding server port from command line", slog.String("port", port))
	}

	// Create router
	r := mux.NewRouter()

	// Server configuration
	serverPort := port
	logger.Info("Starting BSS Metrics API Server", slog.String("port", serverPort))
	logger.Info("JWT Authentication: Enabled", slog.Int("token_duration_hours", cfg.JWT.TokenDuration))
	// Setup routes
	rest.SetupRoutes(r, hdl)

	// Start server
	srv := &http.Server{
		Handler:      r,
		Addr:         serverPort,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Failed to start server, exit", util.LogErrAttr(err))
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	logger.Info("Shutting down server...")
	close(stopPodWatcher)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", util.LogErrAttr(err))
	} else {
		logger.Info("Server exited gracefully")
	}
}
