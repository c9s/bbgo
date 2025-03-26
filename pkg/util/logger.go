package util

import (
	"context"

	log "github.com/sirupsen/logrus"
)

type logKey struct{}

func GetLoggerFromCtxOrFallback(ctx context.Context, entry *log.Entry) *log.Entry {
	if v := ctx.Value(logKey{}); v != nil {
		return v.(*log.Entry)
	}
	return entry
}

func GetLoggerFromCtx(ctx context.Context) *log.Entry {
	if v := ctx.Value(logKey{}); v != nil {
		return v.(*log.Entry)
	}
	return log.NewEntry(log.StandardLogger())
}

func SetLoggerToCtx(ctx context.Context, logger *log.Entry) context.Context {
	ctx = context.WithValue(ctx, logKey{}, logger)
	return ctx
}
