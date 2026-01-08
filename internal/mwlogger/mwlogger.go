// Package mwlogger provides UUID-logging to every request
package mwlogger

import (
	"context"
	"net/http"

	"github.com/wb-go/wbf/ginext"
	"github.com/wb-go/wbf/helpers"
	"github.com/wb-go/wbf/zlog"
)

type loggerWithRequestID struct{}

// NewMWLogger - обёртка для логирования запросов с присвоением UUID каждому запросу и пробросу логгера в контекст запроса
func NewMWLogger(next *ginext.Engine) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fetching/generating UUID for request
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = helpers.CreateUUID()
		}

		// Creating logger
		logger := zlog.Logger.With().
			Str("request_id", reqID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Logger()

		// Putting logger to context
		ctx := context.WithValue(r.Context(), loggerWithRequestID{}, logger)
		r = r.WithContext(ctx)

		// Running handler
		next.ServeHTTP(w, r)
	})
}

// LoggerFromContext extracts logger from context - used in service-layer
func LoggerFromContext(ctx context.Context) zlog.Zerolog {
	if l, ok := ctx.Value(loggerWithRequestID{}).(zlog.Zerolog); ok {
		return l
	}
	return zlog.Logger
}
