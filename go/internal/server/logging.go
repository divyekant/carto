package server

import (
	"log/slog"
	"os"
)

// serverLog is the structured JSON logger for the server package.
// Every log line is a single JSON object written to stdout, making it
// directly ingestible by log aggregators (Datadog, CloudWatch Logs, Loki).
//
// ReplaceAttr maps slog's default "time" key to "ts" for consistency with
// the Carto log schema used across the rest of the stack.
//
// slog.SetDefault is called so that any remaining stdlib log.Printf calls
// (e.g. in cmd_serve.go or third-party libraries) are also routed through
// the JSON handler and formatted consistently.
var serverLog = initServerLogger()

func initServerLogger() *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			// Rename the built-in "time" key to "ts" to maintain
			// backward compatibility with existing log-parser configs.
			if a.Key == slog.TimeKey {
				a.Key = "ts"
			}
			return a
		},
	})
	l := slog.New(h)

	// SetDefault re-routes stdlib log.Printf through the JSON handler so that
	// any call sites that haven't been migrated yet remain consistent with the
	// structured output. This is safe to call from a package-level var
	// initialiser because it only mutates the global slog default.
	slog.SetDefault(l)

	return l
}
