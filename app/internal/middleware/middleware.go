package middleware

import (
	"context"			//Data load and life cicle management
	"log/slog"          //Structured Logging
	"net/http"          //Server tools
	"runtime/debug"     //For Stack Trace
	"time"

	"github.com/google/uuid" //Universally unique identifier
)

// contextKey is a private type to avoid context key collisions across packages.
type contextKey string

const requestIDKey contextKey = "request_id"

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// Middleware is a standard http.Handler wrapper.
type Middleware func(http.Handler) http.Handler

// Chain composes middlewares left-to-right (outermost first).
func Chain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// RequestID injects a unique X-Request-ID into every request context and response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			if id = GetRequestID(r.Context()); id == "" {
				id = uuid.NewString()
			}
		}

		r = r.WithContext(context.WithValue(r.Context(), requestIDKey, id))
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

// Logger records method, path, status, and latency for every request.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK} // &responseWriter is a pointer, ResponseWriter: w means w is wrapped using ResponseWriter 
		start := time.Now()

		next.ServeHTTP(rw, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"latency_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"request_id", GetRequestID(r.Context()),
		)
	})
}

// Recover catches panics and returns 500 without crashing the process.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { // This function will be executed when the current function finishes, even if an error occurs.
			if rec := recover(); rec != nil { //Captures the panic that occurred in the current goroutine
				slog.Error("panic recovered",
					"error", rec,
					"stack", string(debug.Stack()),
					"request_id", GetRequestID(r.Context()),
				)
				http.Error(w, `{"code":500,"message":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseWriter captures the HTTP status code for logging.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}