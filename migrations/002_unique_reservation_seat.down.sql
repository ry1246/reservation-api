ALTER TABLE reservations DROP CONSTRAINT reservations_seat_id_key;

CREATE INDEX idx_reservations_seat_id ON reservations(seat_id);
