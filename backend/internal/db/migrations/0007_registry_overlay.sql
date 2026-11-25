-- The registry's overlay: how this service presents SAM's registry
-- entries, keyed by SAM's IDs, which SAM never sees
-- (docs/design/registry-and-content.md#the-overlay). An entry without a
-- row is shown as SAM defines it.
--
-- In each table, seeded is true while dgs seed wrote the row last, so a
-- later seed may update it; any other change clears it, and the seed then
-- keeps the row as it is.

-- A domain's label, icon and description, in display order. A hidden
-- domain is not shown, nor are its values under it.
CREATE TABLE registry_domains (
    id          text PRIMARY KEY,
    label       text NOT NULL,
    -- A registry icon name of the front end's.
    icon        text NOT NULL,
    description text NOT NULL,
    hidden      boolean NOT NULL DEFAULT false,
    position    integer NOT NULL,
    seeded      boolean NOT NULL,
    updated_at  timestamptz NOT NULL
);

-- A parameter's text and display, in display order. A null column shows
-- SAM's own: its definition, its domain, its unit.
CREATE TABLE registry_parameters (
    id          text PRIMARY KEY,
    label       text NOT NULL,
    definition  text,
    -- What the value means, in one plain sentence; empty for none.
    plain       text NOT NULL,
    -- How a measured or entered value is taken, shown where SAM has no
    -- formula; null for the wording of its source.
    explanation text,
    -- The domain it is shown under, instead of SAM's.
    domain      text,
    -- The unit as shown, such as °C for SAM's degC.
    unit        text,
    source      text NOT NULL CHECK (source IN ('weather_station', 'device', 'manual')),
    -- Changes smaller than this, in the parameter's unit, read as steady.
    noise       double precision CHECK (noise >= 0),
    chart       text CHECK (chart IN ('daily_bars', 'line_band', 'line')),
    position    integer NOT NULL,
    seeded      boolean NOT NULL,
    updated_at  timestamptz NOT NULL
);

-- A device type's label and display, in display order, and the values
-- shown as tiles and charted as cards when it is picked.
CREATE TABLE registry_device_types (
    id          text PRIMARY KEY,
    label       text NOT NULL,
    description text NOT NULL,
    color       text NOT NULL,
    icon        text NOT NULL,
    tiles       text[] NOT NULL,
    cards       text[] NOT NULL,
    position    integer NOT NULL,
    seeded      boolean NOT NULL,
    updated_at  timestamptz NOT NULL
);

-- An observation template's label, description, the domain it is listed
-- under and the device type it is read off, in display order.
CREATE TABLE registry_templates (
    id          text PRIMARY KEY,
    label       text NOT NULL,
    description text NOT NULL,
    domain      text,
    device_type text,
    position    integer NOT NULL,
    seeded      boolean NOT NULL,
    updated_at  timestamptz NOT NULL
);
