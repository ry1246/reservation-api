package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"net/http"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
