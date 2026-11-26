-- What platform admins change in the registry
-- (docs/design/registry-and-content.md): this tenant's own entries, which
-- the outbox sends to SAM, the log of every change, the platform's default
-- alert levels, and how a value is explained.

-- This tenant's own domains, parameters, device types and templates, each
-- as sent to SAM: the whole of SAM's input but its version. A null
-- definition is an entry removed, until SAM has removed it too. The
-- registry shows these until SAM has them.
CREATE TABLE registry_own_entries (
    kind       text NOT NULL CHECK (kind IN ('domain', 'parameter', 'device_type', 'template')),
    id         text NOT NULL,
    definition jsonb,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (kind, id)
);

-- Every change an admin makes to the registry, with their name at the
-- time, newest first.
CREATE TABLE registry_changes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    at         timestamptz NOT NULL,
    actor_id   uuid REFERENCES users ON DELETE SET NULL,
    actor_name text NOT NULL,
    summary    text NOT NULL
);
CREATE INDEX registry_changes_newest ON registry_changes (at DESC, id DESC);

-- The platform's default alert levels, the lowest layer of a device's, in
-- each parameter's own unit, in display order. seeded is as in the
-- overlay's tables.
CREATE TABLE alert_defaults (
    id           text PRIMARY KEY,
    parameter_id text NOT NULL,
    direction    text NOT NULL CHECK (direction IN ('above', 'below')),
    value        double precision NOT NULL,
    level        text NOT NULL CHECK (level IN ('caution', 'warning')),
    title        text NOT NULL,
    enabled      boolean NOT NULL,
    position     integer NOT NULL,
    seeded       boolean NOT NULL,
    updated_at   timestamptz NOT NULL
);

-- How a value is explained: the method and inputs an admin states, with
-- the formula in explanation. Null shows what SAM and the value's source
-- say. A domain of '' shows a value under no domain.
ALTER TABLE registry_parameters
    ADD COLUMN method text CHECK (method IN ('measured', 'derived', 'manual')),
    ADD COLUMN inputs text[];
