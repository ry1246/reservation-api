package main

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
    ctx := context.Background()

    pool, err := newPool(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()

    mux := http.NewServeMux()
    mux.HandleFunc("GET /health", healthHandler)

    log.Println("listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", mux))
}
