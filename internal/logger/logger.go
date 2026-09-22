package logger

import (
	"context"

	"github.com/sirupsen/logrus"
)

type contextKey struct{}
type reqIDContextKey struct{}

var (
	loggerKey contextKey
	reqKey    reqIDContextKey
	fallback  = logrus.New()
)

func SetFallback(log *logrus.Logger) {
	if log != nil {
		fallback = log
	}
}

func WithLogger(ctx context.Context, entry *logrus.Entry) context.Context {
	if entry == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerKey, entry)
}

func WithFields(ctx context.Context, fields logrus.Fields) context.Context {
	return WithLogger(ctx, FromContext(ctx).WithFields(fields))
}

func FromContext(ctx context.Context) *logrus.Entry {
	if ctx != nil {
		if entry, ok := ctx.Value(loggerKey).(*logrus.Entry); ok && entry != nil {
			return entry
		}
	}
	return logrus.NewEntry(fallback)
}

func WithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, reqKey, reqID)
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx != nil {
		if id, ok := ctx.Value(reqKey).(string); ok {
			return id
		}
	}
	return ""
}
