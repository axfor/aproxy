package observability

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	*zap.Logger
	redactParams bool
}

func NewLogger(level string, format string, redactParams bool) (*Logger, error) {
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	var config zap.Config
	if format == "json" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}

	config.Level = zap.NewAtomicLevelAt(zapLevel)
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{
		Logger:       logger,
		redactParams: redactParams,
	}, nil
}

func (l *Logger) LogQuery(sessionID, user, clientIP, query string, duration float64, rowsAffected int64, err error) {
	if l.redactParams {
		query = l.redactQuery(query)
	}

	fields := []zap.Field{
		zap.String("session_id", sessionID),
		zap.String("user", user),
		zap.String("client_ip", clientIP),
		zap.String("query", query),
		zap.Float64("duration_seconds", duration),
		zap.Int64("rows_affected", rowsAffected),
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
		l.Error("query_error", fields...)
	} else {
		l.Info("query_executed", fields...)
	}
}

func (l *Logger) LogConnection(sessionID, user, clientIP string, connected bool) {
	if connected {
		l.Info("client_connected",
			zap.String("session_id", sessionID),
			zap.String("user", user),
			zap.String("client_ip", clientIP),
		)
	} else {
		l.Info("client_disconnected",
			zap.String("session_id", sessionID),
			zap.String("user", user),
			zap.String("client_ip", clientIP),
		)
	}
}

func (l *Logger) LogError(sessionID, user, clientIP, errorType string, err error) {
	l.Error("error",
		zap.String("session_id", sessionID),
		zap.String("user", user),
		zap.String("client_ip", clientIP),
		zap.String("error_type", errorType),
		zap.Error(err),
	)
}

func (l *Logger) redactQuery(query string) string {
	if len(query) > 100 {
		return query[:100] + "... [REDACTED]"
	}
	return "[REDACTED]"
}
