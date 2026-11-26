-- The answer to each record of an offline sync batch SAM took, by the
-- person and the client's ID, so a retry gets it back without asking SAM
-- again (docs/design/labs-and-readings.md#offline-sync).
CREATE TABLE synced_records (
    user_id     uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    client_id   text NOT NULL,
    observation jsonb NOT NULL,
    clock_flags text[] NOT NULL,
    synced_at   timestamptz NOT NULL,
    PRIMARY KEY (user_id, client_id)
);
