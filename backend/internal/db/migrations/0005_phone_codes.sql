-- Signing in with a code texted to a phone, and the limits that keep the
-- codes from being guessed or the SMS budget from being spent
-- (docs/design/accounts-and-sign-in.md#phone-codes).

-- A phone number is unique only once verified.
CREATE UNIQUE INDEX users_verified_phone ON users (phone) WHERE phone_verified;

-- A code sent to a number: its hash only, for one purpose, and for one
-- signed-in user when it verifies their phone.
CREATE TABLE phone_challenges (
    id         uuid PRIMARY KEY,
    phone      text NOT NULL,
    purpose    text NOT NULL CHECK (purpose IN ('sign_in', 'verify_phone')),
    user_id    uuid REFERENCES users (id) ON DELETE CASCADE,
    code_hash  bytea NOT NULL,
    created_at timestamptz NOT NULL,
    resend_at  timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    -- Set when the code is used, or when a newer code replaces it.
    ended_at   timestamptz
);

CREATE INDEX phone_challenges_live ON phone_challenges (phone, purpose) WHERE ended_at IS NULL;

-- A number's attempt budget for a purpose: the attempts made while its
-- codes have lived. A new code extends the window and keeps the count; only
-- once every code has expired does the count start again.
CREATE TABLE phone_attempts (
    phone           text NOT NULL,
    purpose         text NOT NULL,
    attempts        integer NOT NULL,
    window_ends_at  timestamptz NOT NULL,
    PRIMARY KEY (phone, purpose)
);

-- Sliding-window limits, counted here so they hold across replicas. Taking
-- a limit locks its key's row, so two requests never both take the last
-- place; each use is one event, kept for the window.
CREATE TABLE rate_limits (
    key  text PRIMARY KEY
);

CREATE TABLE rate_events (
    key text NOT NULL REFERENCES rate_limits (key),
    at  timestamptz NOT NULL
);

CREATE INDEX rate_events_key_at ON rate_events (key, at);
