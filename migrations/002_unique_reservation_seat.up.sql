DELETE FROM reservations
WHERE id NOT IN (
    SELECT min(id) FROM reservations GROUP BY seat_id
);

DROP INDEX idx_reservations_seat_id;

ALTER TABLE reservations
    ADD CONSTRAINT reservations_seat_id_key UNIQUE (seat_id);

