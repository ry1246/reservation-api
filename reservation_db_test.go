package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func newTestServer(t *testing.T) *server {
	t.Helper()

	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL is not set")
	}

	pool, err := newPool(context.Background())
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return &server{pool: pool}
}

// テスト専用のイベントと座席を1つ作り、終了時に片付ける。
func createTestSeat(t *testing.T, s *server) int64 {
	t.Helper()
	ctx := context.Background()

	var eventID, seatID int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO events (name, starts_at) VALUES ('test-event', now()) RETURNING id`,
	).Scan(&eventID)
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO seats (event_id, label) VALUES ($1, 'T1') RETURNING id`, eventID,
	).Scan(&seatID)
	if err != nil {
		t.Fatalf("insert seat: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		s.pool.Exec(ctx, "DELETE FROM reservations WHERE seat_id = $1", seatID)
		s.pool.Exec(ctx, "DELETE FROM seats WHERE id = $1", seatID)
		s.pool.Exec(ctx, "DELETE FROM events WHERE id = $1", eventID)
	})

	return seatID
}

func TestCreateReservation_Concurrent(t *testing.T) {
	s := newTestServer(t)
	seatID := createTestSeat(t, s)

	const n = 50
	codes := make([]int, n)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start

			body := fmt.Sprintf(`{"seat_id":%d,"user_name":"user%d"}`, seatID, i)
			req := httptest.NewRequest(http.MethodPost, "/reservations", strings.NewReader(body))
			rec := httptest.NewRecorder()

			s.createReservation(rec, req)
			codes[i] = rec.Code
		}(i)
	}

	close(start)
	wg.Wait()

	var created, conflict int
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflict++
		}
	}
	if created != 1 {
		t.Errorf("201 = %d, want 1 (codes: %v)", created, codes)
	}
	if conflict != n-1 {
		t.Errorf("409 = %d, want %d (codes: %v)", conflict, n-1, codes)
	}

	var rows int
	err := s.pool.QueryRow(context.Background(),
		"SELECT count(*) FROM reservations WHERE seat_id = $1", seatID,
	).Scan(&rows)
	if err != nil {
		t.Fatalf("count reservations: %v", err)
	}
	if rows != 1 {
		t.Errorf("reservations rows = %d, want 1", rows)
	}
}
