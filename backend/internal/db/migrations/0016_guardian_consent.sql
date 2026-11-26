-- Guardian consent by link (docs/design/classrooms-and-permissions.md#guardian-consent).

-- A consent link sent to a minor's guardian. Only the token's hash is
-- kept. Asking again voids the live link; a minor has one live link at
-- most. The contact is a lower-case email or an E.164 number.
CREATE TABLE guardian_links (
    id               uuid PRIMARY KEY,
    user_id          uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    guardian_name    text NOT NULL,
    guardian_contact text NOT NULL,
    token_hash       bytea NOT NULL UNIQUE,
    requested_at     timestamptz NOT NULL,
    voided_at        timestamptz
);

CREATE UNIQUE INDEX guardian_links_live ON guardian_links (user_id) WHERE voided_at IS NULL;

-- Each guardian decision about a minor, newest last: the consent text's
-- version the guardian was shown and the rounding of positions it
-- promised.
CREATE TABLE consent_records (
    id                  uuid PRIMARY KEY,
    user_id             uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    method              text NOT NULL CHECK (method IN ('link', 'paper')),
    link_id             uuid REFERENCES guardian_links ON DELETE SET NULL,
    decision            text NOT NULL CHECK (decision IN ('given', 'withdrawn')),
    guardian_name       text NOT NULL,
    text_version        integer NOT NULL,
    coordinate_decimals integer NOT NULL,
    decided_at          timestamptz NOT NULL
);

CREATE INDEX consent_records_user ON consent_records (user_id, decided_at DESC);
