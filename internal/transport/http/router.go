package httpadapter

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code and response size.
type responseWriter struct {
	http.ResponseWriter
	status      int
	size        int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

func (rw *responseWriter) Unwrap() http.ResponseWriter { return rw.ResponseWriter }

func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		if !rw.wroteHeader {
			rw.WriteHeader(http.StatusOK)
		}
		flusher.Flush()
	}
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not support hijacking")
	}
	return hijacker.Hijack()
}

func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := rw.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

// RequestLogger is an idiomatic middleware that logs HTTP request lifecycles using slog.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now().UTC()

			// Default status to 200 OK if WriteHeader isn't explicitly called
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			logger.InfoContext(r.Context(), "HTTP request handled",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"bytes", rw.size,
				"duration_ms", duration.Milliseconds(),
				"remote_ip", r.RemoteAddr,
			)
		})
	}
}
