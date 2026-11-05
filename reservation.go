package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type Reservation struct {
	ID        int64     `json:"id"`
	SeatID    int64     `json:"seat_id"`
	UserName  string    `json:"user_name"`
	CreatedAt time.Time `json:"created_at"`
}

type createReservationRequest struct {
	SeatID   int64  `json:"seat_id"`
	UserName string `json:"user_name"`
}

func (s *server) createReservation(w http.ResponseWriter, r *http.Request) {
	// 1. JSON をデコード
	var req createReservationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// 2. 最低限のチェック
	if req.SeatID <= 0 || req.UserName == "" {
		http.Error(w, "seat_id and user_name are required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// 3. 座席が存在するか確認
	var one int
	err := s.pool.QueryRow(ctx,
		"SELECT 1 FROM seats WHERE id = $1", req.SeatID,
	).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "seat not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("check seat: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// 4. すでに予約があるか確認
	var reserved bool
	err = s.pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM reservations WHERE seat_id = $1)", req.SeatID,
	).Scan(&reserved)
	if err != nil {
		log.Printf("check reservation: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if reserved {
		http.Error(w, "seat already reserved", http.StatusConflict)
		return
	}

	// 5. reservations に INSERT して、採番された行を受け取る
	var rsv Reservation
	err = s.pool.QueryRow(ctx,
		`INSERT INTO reservations (seat_id, user_name) VALUES ($1, $2)
             RETURNING id, seat_id, user_name, created_at`,
		req.SeatID, req.UserName,
	).Scan(&rsv.ID, &rsv.SeatID, &rsv.UserName, &rsv.CreatedAt)
	if err != nil {
		log.Printf("insert reservation: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// 6. 201 で返す
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(rsv); err != nil {
		log.Printf("encode reservation: %v", err)
	}
}
