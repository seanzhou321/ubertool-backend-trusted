package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

var defaultLogger *slog.Logger

// Initialize sets up the global logger with the specified level and format
func Initialize(level, format string) {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if strings.ToLower(format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

// Get returns the default logger
func Get() *slog.Logger {
	if defaultLogger == nil {
		// Initialize with default settings if not yet initialized
		Initialize("info", "text")
	}
	return defaultLogger
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}

// DebugContext logs a debug message with context
func DebugContext(ctx context.Context, msg string, args ...any) {
	Get().DebugContext(ctx, msg, args...)
}

// InfoContext logs an info message with context
func InfoContext(ctx context.Context, msg string, args ...any) {
	Get().InfoContext(ctx, msg, args...)
}

// WarnContext logs a warning message with context
func WarnContext(ctx context.Context, msg string, args ...any) {
	Get().WarnContext(ctx, msg, args...)
}

// ErrorContext logs an error message with context
func ErrorContext(ctx context.Context, msg string, args ...any) {
	Get().ErrorContext(ctx, msg, args...)
}

// WithMethod returns a logger with method name attached
func WithMethod(methodName string) *slog.Logger {
	return Get().With("method", methodName)
}

// WithService returns a logger with service name attached
func WithService(serviceName string) *slog.Logger {
	return Get().With("service", serviceName)
}

// EnterMethod logs method entry (process tracking)
func EnterMethod(methodName string, args ...any) {
	allArgs := append([]any{"method", methodName, "event", "enter"}, args...)
	Get().Debug("→ Method entered", allArgs...)
}

// ExitMethod logs method exit (process tracking)
func ExitMethod(methodName string, args ...any) {
	allArgs := append([]any{"method", methodName, "event", "exit"}, args...)
	Get().Debug("← Method exited", allArgs...)
}

// ExitMethodWithError logs method exit with error (process tracking)
func ExitMethodWithError(methodName string, err error, args ...any) {
	allArgs := append([]any{"method", methodName, "event", "exit", "error", err}, args...)
	Get().Error("← Method exited with error", allArgs...)
}

// DatabaseCall logs database operation (debug log for external resources)
func DatabaseCall(operation, query string, args ...any) {
	allArgs := append([]any{"operation", operation, "query", query}, args...)
	Get().Debug("→ Database call", allArgs...)
}

// DatabaseResult logs database operation result (debug log for external resources)
func DatabaseResult(operation string, rowsAffected int64, err error, args ...any) {
	allArgs := append([]any{"operation", operation, "rows_affected", rowsAffected}, args...)
	if err != nil {
		allArgs = append(allArgs, "error", err)
		Get().Error("← Database call failed", allArgs...)
	} else {
		Get().Debug("← Database call succeeded", allArgs...)
	}
}

// ExternalServiceCall logs external service call (debug log for external resources)
func ExternalServiceCall(service, operation string, args ...any) {
	allArgs := append([]any{"service", service, "operation", operation}, args...)
	Get().Debug("→ External service call", allArgs...)
}

// ExternalServiceResult logs external service result (debug log for external resources)
func ExternalServiceResult(service, operation string, err error, args ...any) {
	allArgs := append([]any{"service", service, "operation", operation}, args...)
	if err != nil {
		allArgs = append(allArgs, "error", err)
		Get().Error("← External service call failed", allArgs...)
	} else {
		Get().Debug("← External service call succeeded", allArgs...)
	}
}
