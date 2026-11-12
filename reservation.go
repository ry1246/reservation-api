package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	// 2. 最低限のチェック
	if req.SeatID <= 0 || req.UserName == "" || len(req.UserName) > 100 {
		writeError(w, http.StatusBadRequest, "seat_id and user_name(1-100 chars) are required")
		return
	}

	ctx := r.Context()

	// 3. トランザクション開始
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		log.Printf("begin tx: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer tx.Rollback(ctx)

	// 4. 座席の行をロックしつつ、存在確認
	var one int
	err = tx.QueryRow(ctx,
		"SELECT 1 FROM seats WHERE id = $1 FOR UPDATE", req.SeatID,
	).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "seat not found")
		return
	}
	if err != nil {
		log.Printf("lock seat: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// 5. すでに予約があるか確認
	var reserved bool
	err = tx.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM reservations WHERE seat_id = $1)", req.SeatID,
	).Scan(&reserved)
	if err != nil {
		log.Printf("check reservation: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if reserved {
		writeError(w, http.StatusConflict, "seat already reserved")
		return
	}

	// 6. reservations に INSERT して、採番された行を受け取る
	var rsv Reservation
	err = tx.QueryRow(ctx,
		`INSERT INTO reservations (seat_id, user_name) VALUES ($1, $2)
             RETURNING id, seat_id, user_name, created_at`,
		req.SeatID, req.UserName,
	).Scan(&rsv.ID, &rsv.SeatID, &rsv.UserName, &rsv.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "seat already reserved")
			return
		}
		log.Printf("insert reservations: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// 7. コミットしてロックを解放
	if err := tx.Commit(ctx); err != nil {
		log.Printf("commit: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// 8. 201 で返す
	writeJSON(w, http.StatusCreated, rsv)
}

func (s *server) cancelReservation(w http.ResponseWriter, r *http.Request) {
	// 1. パスの {id} を int64 に変換
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid reservation id")
		return
	}

	// 2. DELETE を実行
	tag, err := s.pool.Exec(r.Context(),
		"DELETE FROM reservations WHERE id = $1", id,
	)
	if err != nil {
		log.Printf("delete reservation: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// 3. 消えた行がなければ 404
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "reservation not found")
		return
	}

	// 4. 成功は 204 No Content
	w.WriteHeader(http.StatusNoContent)
}
