package httpadapter_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpadapter "example.com/notes/internal/transport/http"
)

func TestRequestLogger(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body       string
		wantStatus int
		wantBytes  int
	}{
		{"implicit OK", 0, "hello", 200, 5},
		{"explicit status", 201, "created", 201, 7},
		{"empty response", 204, "", 204, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs strings.Builder
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			h := httpadapter.RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				if tc.body != "" {
					_, _ = io.WriteString(w, tc.body)
				}
			}))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/notes", nil))
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if !strings.Contains(logs.String(), `status=`+itoa(tc.wantStatus)) || !strings.Contains(logs.String(), `bytes=`+itoa(tc.wantBytes)) {
				t.Fatalf("log lacks status/bytes: %s", logs.String())
			}
		})
	}
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }
