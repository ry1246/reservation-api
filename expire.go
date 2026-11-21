package main

import (
	"context"
	"fmt"
)

func (s *server) deleteExpiredReservations(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		"DELETE FROM reservations WHERE expires_at < now()")
	if err != nil {
		return 0, fmt.Errorf("delete expired reservations: %w", err)
	}
	return tag.RowsAffected(), nil
}
