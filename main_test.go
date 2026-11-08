package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"status":"ok"`) {
		t.Errorf("body = %q, want it to contain %q", body, `"status":"ok"`)
	}
}

func TestCreateEvent_BadRequest(t *testing.T) {
	s := &server{}

	tests := []struct {
		name string
		body string
	}{
		{"壊れたJSON", `{`},
		{"nameが空", `{"name":"","starts_at":"2026-12-01-T10:00:00Z","seat_count":10}`},
		{"starts_atが無い", `{"name":"live","seat_count":10}`},
		{"seat_countが0", `{"name":"live","starts_at":"2026-12-01T10:00:00Z","seat_count":0}`},
		{"seat_countが10001", `{"name":"live","starts_at""2026-12-01T10:00:00Z","seat_count":10001}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			s.createEvent(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}

			var resp errorResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode body %v", err)
			}
			if resp.Error == "" {
				t.Error("error message is empty")
			}
		})
	}
}
