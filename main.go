package main

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type server struct {
	pool *pgxpool.Pool
}

func main() {
	ctx := context.Background()

	pool, err := newPool(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	s := &server{pool: pool}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /events", s.listEvents)
	mux.HandleFunc("POST /events", s.createEvent)
	mux.HandleFunc("GET /events/{id}/seats", s.listSeats)
	mux.HandleFunc("POST /reservations", s.createReservation)
	mux.HandleFunc("DELETE /reservations/{id}", s.cancelReservation)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
