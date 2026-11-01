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
