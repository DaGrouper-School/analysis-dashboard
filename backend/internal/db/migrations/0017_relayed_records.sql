-- Each record this service relayed to SAM for a person
-- (docs/design/labs-and-readings.md#relaying-readings): SAM's reading ID,
-- the form it came through, who recorded it and who asked for credit. A
-- name appears with a record only with credit, and never a minor's.
CREATE TABLE relayed_records (
    reading_id     uuid PRIMARY KEY,
    form           text NOT NULL CHECK (form IN ('upload', 'observation', 'batch')),
    lab_id         uuid NOT NULL REFERENCES labs ON DELETE CASCADE,
    device_id      uuid REFERENCES devices ON DELETE SET NULL,
    recorded_by    uuid REFERENCES users ON DELETE SET NULL,
    credit_user_id uuid REFERENCES users ON DELETE SET NULL,
    received_at    timestamptz NOT NULL
);

CREATE INDEX relayed_records_credit ON relayed_records (credit_user_id) WHERE credit_user_id IS NOT NULL;
