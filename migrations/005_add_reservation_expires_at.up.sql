ALTER TABLE reservations ADD COLUMN expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '15 minutes';
