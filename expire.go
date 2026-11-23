package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

func (s *server) deleteExpiredReservations(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		"DELETE FROM reservations WHERE expires_at < now()")
	if err != nil {
		return 0, fmt.Errorf("delete expired reservations: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (s *server) runExpirer(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.deleteExpiredReservations(ctx)
			if err != nil {
				log.Printf("expire reservations: %v", err)
				continue
			}
			if n > 0 {
				log.Printf("expired %d reservations", n)
			}
		}
	}
}
