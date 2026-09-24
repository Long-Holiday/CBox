package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	pkgApi "cbox/pkg/api"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered in HTTP handler", "panic", rec, "path", r.URL.Path)
					wrapped.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(wrapped).Encode(pkgApi.ErrorResponse{
						Error: "internal server error",
					})
				}
				logger.Debug("HTTP request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", wrapped.statusCode,
					"duration", time.Since(start),
				)
			}()

			next.ServeHTTP(wrapped, r)
		})
	}
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func WriteError(w http.ResponseWriter, status int, err error) {
	msg := "unknown error"
	if err != nil {
		msg = err.Error()
	}
	WriteJSON(w, status, pkgApi.ErrorResponse{
		Error: msg,
	})
}
