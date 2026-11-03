package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Event struct {
	ID       int64     `json:"id"`
	Name     string    `json:"name"`
	StartsAt time.Time `json:"starts_at"`
}

func (s *server) listEvents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(),
		"SELECT id, name, starts_at FROM events ORDER BY starts_at")
	if err != nil {
		log.Printf("query events: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Name, &e.StartsAt); err != nil {
			log.Printf("scan event: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate events: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(events); err != nil {
		log.Printf("encode events: %v", err)
	}
}

type createEventRequest struct {
  Name      string    `json:"name"`
  StartsAt  time.Time `json:"starts_at"`
  SeatCount int       `json:"seat_count"`
}

func (s *server) createEvent(w http.ResponseWriter, r *http.Request) {
    // 1. JSON をデコード 壊れたJSONは400
    var req createEventRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid json", http.StatusBadRequest)
        return
    }

    // 2. 最低限のチェック
    if req.Name == "" || req.StartsAt.IsZero() || req.SeatCount <= 0 || req.SeatCount > 10000 {
        http.Error(w, "name, starts_at, seat_count(1-10000) are required", http.StatusBadRequest)
        return
    }

    ctx := r.Context()

    // 3. トランザクション開始
    tx, err := s.pool.Begin(ctx)
    if err != nil {
        log.Printf("begin tx: %v", err)
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }
    defer tx.Rollback(ctx)

    // 4. events に INSERT して、採番された id などを受け取る
    var e Event
    err = tx.QueryRow(ctx,
        `INSERT INTO events (name, starts_at) VALUES ($1, $2)
        RETURNING id, name, starts_at`,
    req.Name, req.StartsAt,
    ).Scan(&e.ID, &e.Name, &e.StartsAt)
    if err != nil {
        log.Printf("insert event: %v", err)
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }

    // 5. seats をまとめて INSERT(S1, S2, ... SN)
    _, err = tx.Exec(ctx,
        `INSERT INTO seats (event_id, label)
         SELECT $1::bigint, 'S' || g FROM generate_series(1, $2::int) AS g`,
        e.ID, req.SeatCount,
    )
    if err != nil {
        log.Printf("insert seats: %v", err)
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }

    // 6. コミット
    if err := tx.Commit(ctx); err != nil {
        log.Printf("commit: %v", err)
        http.Error(w, "internal server error", http.StatusInternalServerError)
        return
    }

    // 7. 201 で返す(Header => WriteHeader => Encode の順)
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    if err := json.NewEncoder(w).Encode(e); err != nil {
        log.Printf("encode event: %v", err)
    }
}
