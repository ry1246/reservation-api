CREATE TABLE events (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        TEXT NOT NULL,
    starts_at   TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE seats (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id    BIGINT NOT NULL REFERENCES events(id),
    label       TEXT NOT NULL,
    UNIQUE (event_id, label)
);

CREATE TABLE reservations (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    seat_id     BIGINT NOT NULL REFERENCES seats(id),
    user_name   TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_seats_event_id ON seats(event_id);
CREATE INDEX idx_reservations_seat_id ON reservations(seat_id);
