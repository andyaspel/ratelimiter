package ratelimiter

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggerMiddleware_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := RequestLoggerMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	out := buf.String()
	if !strings.Contains(out, "http request") {
		t.Fatalf("expected log output to contain message, got %q", out)
	}
	if !strings.Contains(out, "path=/health") {
		t.Fatalf("expected path to be logged, got %q", out)
	}
	if !strings.Contains(out, "status=201") {
		t.Fatalf("expected status to be logged, got %q", out)
	}
}
