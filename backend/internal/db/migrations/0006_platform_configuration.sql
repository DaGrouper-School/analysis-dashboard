-- The platform's configuration: the lists other rows refer to, the
-- settings the server applies, and the log of every change to them
-- (docs/design/registry-and-content.md#platform-configuration).
--
-- In each table, seeded is true while dgs seed wrote the row last, so a
-- later seed may update it; any other change clears it, and the seed then
-- keeps the row as it is.

-- A classroom's stage on the way to operating, in display order.
CREATE TABLE stages (
    id          text PRIMARY KEY,
    label       text NOT NULL,
    description text NOT NULL,
    -- The marker and badge color, such as #f6921e.
    color       text NOT NULL,
    position    integer NOT NULL,
    seeded      boolean NOT NULL,
    updated_at  timestamptz NOT NULL
);

-- The regions the directory groups classrooms by, in display order.
CREATE TABLE regions (
    id         text PRIMARY KEY,
    label      text NOT NULL,
    position   integer NOT NULL,
    seeded     boolean NOT NULL,
    updated_at timestamptz NOT NULL
);

-- The host application's readiness questions, in order. A question with
-- answers is retired rather than deleted, so earlier applications keep
-- their meaning.
CREATE TABLE host_questions (
    id         text PRIMARY KEY,
    label      text NOT NULL,
    help       text NOT NULL,
    kind       text NOT NULL CHECK (kind IN ('yes_no', 'number', 'text')),
    required   boolean NOT NULL,
    position   integer NOT NULL,
    retired_at timestamptz,
    seeded     boolean NOT NULL,
    updated_at timestamptz NOT NULL
);

-- One checked value per setting: core_parameters, default_terms, privacy.
CREATE TABLE platform_settings (
    key        text PRIMARY KEY,
    value      jsonb NOT NULL,
    seeded     boolean NOT NULL,
    updated_at timestamptz NOT NULL
);

-- Every change to the platform's configuration, newest first: who made
-- it, named as they were at the time, and what it was.
CREATE TABLE configuration_changes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    at         timestamptz NOT NULL,
    -- The platform admin who made it; null for the operator and the seed.
    actor_id   uuid REFERENCES users (id) ON DELETE SET NULL,
    actor_name text NOT NULL,
    kind       text NOT NULL,
    subject    text NOT NULL,
    summary    text NOT NULL
);

CREATE INDEX configuration_changes_newest ON configuration_changes (at DESC, id DESC);
