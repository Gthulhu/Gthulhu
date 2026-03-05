package logger

import (
	"context"
	"os"

	"github.com/rs/zerolog"
)

func InitLogger() *zerolog.Logger {
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05"}

	logger := zerolog.New(consoleWriter).
		With().
		Timestamp().
		Caller().
		Logger()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	zerolog.DefaultContextLogger = &logger
	return &logger
}

func Logger(ctx context.Context) *zerolog.Logger {
	return zerolog.Ctx(ctx)
}
