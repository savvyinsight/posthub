// HTTP middleware for the posthub API.
//
// Provides structured request logging with request ID propagation.
// The logger is stored in request context so downstream handlers
// can extract it via logger.FromContext.
package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/savvyinsight/posthub/internal/logger"
)

// RequestLoggerMiddleware returns middleware that logs each request
// with structured fields and stores the logger in context.
func RequestLoggerMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Store logger with request_id in context for downstream handlers.
			requestID := middleware.GetReqID(r.Context())
			ctx := logger.WithRequestID(r.Context(), requestID)
			r = r.WithContext(ctx)

			defer func() {
				log.Info("request completed",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Int("status", ww.Status()),
					zap.Int("bytes", ww.BytesWritten()),
					zap.Int64("duration_ms", time.Since(start).Milliseconds()),
					zap.String("request_id", requestID),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
