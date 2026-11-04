package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

type Seat struct {
	ID       int64  `json:"id"`
	Label    string `json:"label"`
	Reserved bool   `json:"reserved"`
}

func (s *server) listSeats(w http.ResponseWriter, r *http.Request) {
	eventID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	rows, err := s.pool.Query(r.Context(),
		`SELECT s.id, s.label, (r.id IS NOT NULL) AS reserved
         FROM seats s
         LEFT JOIN reservations r ON r.seat_id = s.id
         WHERE s.event_id = $1
         ORDER BY s.id`,
		eventID,
	)
	if err != nil {
		log.Printf("query seats: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	seats := []Seat{}
	for rows.Next() {
		var st Seat
		if err := rows.Scan(&st.ID, &st.Label, &st.Reserved); err != nil {
			log.Printf("scan seat: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		seats = append(seats, st)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate seats: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(seats); err != nil {
		log.Printf("encode seats: %v", err)
	}
}
