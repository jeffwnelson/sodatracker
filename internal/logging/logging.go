package logging

import (
	"log/slog"
	"os"
)

// Setup configures the default slog logger.
// format="json" uses JSONHandler (for Lambda/CloudWatch/Docker).
// format="text" uses TextHandler (for local dev).
func Setup(format string) *slog.Logger {
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
