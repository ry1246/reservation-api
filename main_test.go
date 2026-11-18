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
		{"nameが空", `{"name":"","starts_at":"2026-12-01T10:00:00Z","seat_count":10}`},
		{"starts_atが無い", `{"name":"live","seat_count":10}`},
		{"seat_countが0", `{"name":"live","starts_at":"2026-12-01T10:00:00Z","seat_count":0}`},
        {"seat_countが10001", `{"name":"live","starts_at":"2026-12-01T10:00:00Z","seat_count":10001}`},
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

func TestCreateReservation_BadRequest(t *testing.T) {
    s := &server{}

    tests := []struct {
        name    string
        body    string
        idemKey string
    }{
        {"壊れたJSON", `{`, ""},
        {"seat_idがない", `{"user_name":"a"}`, ""},
        {"seat_idが0", `{"seat_id":0,"user_name":"a"}`, ""},
        {"user_nameが空", `{"seat_id":1,"user_name":""}`,""},
        {"user_nameが101文字", `{"seat_id":1,"user_name":"` + strings.Repeat("a", 101) + `"}`, ""},
        {"Idempotency-Keyが256文字", `{"seat_id":1,"user_name":"a"}`, strings.Repeat("k", 256)},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest(http.MethodPost, "/reservations", strings.NewReader(tt.body))
            if tt.idemKey != "" {
                req.Header.Set("Idempotency-Key", tt.idemKey)
            }
            rec := httptest.NewRecorder()

            s.createReservation(rec, req)

            if  rec.Code != http.StatusBadRequest {
                t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
            }

            var resp errorResponse
            if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
                t.Fatalf("decode body: %v", err)
            }
            if resp.Error == "" {
                t.Error("error message is empty")
            }
        })
    }
}
