-- Signing in with Google, and sessions (docs/design/accounts-and-sign-in.md):
-- the account fields the front end's User carries, the Google identities
-- accounts are linked to, the sessions their cookies name, and the demo
-- accounts demo mode signs in by subject.

ALTER TABLE users
    ADD COLUMN auth_provider        text NOT NULL DEFAULT 'google' CHECK (auth_provider IN ('google', 'phone')),
    ADD COLUMN full_name            text NOT NULL DEFAULT '',
    ADD COLUMN email                text,
    -- An email is unique only when verified, that is, when it came from Google.
    ADD COLUMN email_verified       boolean NOT NULL DEFAULT false,
    ADD COLUMN phone                text,
    ADD COLUMN phone_verified       boolean NOT NULL DEFAULT false,
    ADD COLUMN avatar_url           text,
    ADD COLUMN role                 text NOT NULL DEFAULT 'community_scientist'
                                    CHECK (role IN ('community_scientist', 'custodian', 'facilitator', 'institution_head', 'researcher')),
    ADD COLUMN platform_admin       boolean NOT NULL DEFAULT false,
    ADD COLUMN address              jsonb,
    ADD COLUMN onboarding_completed boolean NOT NULL DEFAULT false,
    ADD COLUMN public_profile       boolean NOT NULL DEFAULT false,
    ADD COLUMN updated_at           timestamptz;

UPDATE users SET updated_at = created_at;
ALTER TABLE users ALTER COLUMN updated_at SET NOT NULL;

CREATE UNIQUE INDEX users_verified_email ON users (lower(email)) WHERE email_verified;

-- A Google identity, linked to its account by Google's subject, so a change
-- of email at Google keeps the account.
CREATE TABLE google_identities (
    subject    text PRIMARY KEY,
    user_id    uuid NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    email      text,
    created_at timestamptz NOT NULL
);

-- A session: the cookie holds an opaque token, this table only its hash.
-- It ends at expires_at, which each use moves on, at most once an hour.
CREATE TABLE sessions (
    token_hash  bytea PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL,
    extended_at timestamptz NOT NULL,
    expires_at  timestamptz NOT NULL
);

CREATE INDEX sessions_user ON sessions (user_id);
CREATE INDEX sessions_expiry ON sessions (expires_at);

-- The Google accounts demo mode offers, signed in by subject instead of a
-- Google token. The demo tool fills it in development and test; it stays
-- empty everywhere else.
CREATE TABLE demo_accounts (
    subject    text PRIMARY KEY,
    name       text NOT NULL,
    email      text NOT NULL,
    type       text NOT NULL,
    avatar_url text
);
