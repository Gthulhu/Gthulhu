package middleware

import (
	"bytes"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/Gthulhu/api/pkg/logger"
	"github.com/rs/xid"
)

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = xid.New().String()
		}
		start := time.Now()
		log := logger.Logger(ctx).With().
			Str("method", r.Method).Str("req_id", reqID).
			Str("url", r.URL.String()).Logger()

		defer func() {
			if err := recover(); err != nil {
				log.Error().Interface("panic", err).Msgf("Recovered from panic, stack trace: %s", string(debug.Stack()))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		ctx = log.WithContext(ctx)
		r = r.WithContext(ctx)
		responseWriter := NewResponseWriter(w)
		next.ServeHTTP(responseWriter, r)
		cost := time.Since(start)
		log = log.With().
			Int("cost_msec", int(cost.Milliseconds())).
			Logger()
		if responseWriter.statusCode >= 500 {
			log.Error().
				Int("status_code", responseWriter.statusCode).
				Str("response_body", responseWriter.responseBody.String()).
				Msg("Request completed with server error")
		} else if responseWriter.statusCode >= 400 {
			log.Warn().
				Int("status_code", responseWriter.statusCode).
				Str("response_body", responseWriter.responseBody.String()).
				Msg("Request completed with client error")
		} else {
			log.Info().
				Int("status_code", responseWriter.statusCode).
				Msg("Request completed successfully")
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	responseBody bytes.Buffer
	statusCode   int
}

func NewResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.responseBody.Write(b)
	return rw.ResponseWriter.Write(b)
}
