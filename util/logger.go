package util

import (
	"context"
	"log/slog"
	"os"

	"github.com/pkg/errors"
)

var logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

var (
	loggerCtxKey = struct{}{}
)

func AddLoggerToCtx(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey, logger)
}

func LoggerFromCtx(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggerCtxKey).(*slog.Logger)
	if !ok || logger == nil {
		return GetLogger()
	}
	return logger
}

func GetLogger() *slog.Logger {
	return logger
}

func LogErrAttr(err error) slog.Attr {
	return slog.String("error", errors.WithStack(err).Error())
}
