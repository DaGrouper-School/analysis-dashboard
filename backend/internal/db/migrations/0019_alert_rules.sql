-- Alert levels above the platform defaults
-- (docs/design/dashboards-and-public-site.md#alert-levels): a device's
-- own, or a classroom's for every device of a sensor type, in the
-- parameter's own unit, with who changed each last.
CREATE TABLE alert_rules (
    id           uuid PRIMARY KEY,
    scope        text NOT NULL CHECK (scope IN ('device', 'type')),
    device_id    uuid REFERENCES devices ON DELETE CASCADE,
    classroom_id uuid REFERENCES classrooms ON DELETE CASCADE,
    device_type  text,
    parameter_id text NOT NULL,
    direction    text NOT NULL CHECK (direction IN ('above', 'below')),
    value        double precision NOT NULL,
    level        text NOT NULL CHECK (level IN ('caution', 'warning')),
    title        text NOT NULL,
    enabled      boolean NOT NULL,
    updated_by   uuid REFERENCES users ON DELETE SET NULL,
    updated_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL,
    CHECK (scope = 'device' AND device_id IS NOT NULL AND classroom_id IS NULL AND device_type IS NULL
        OR scope = 'type' AND device_id IS NULL AND classroom_id IS NOT NULL AND device_type IS NOT NULL)
);

CREATE INDEX alert_rules_device ON alert_rules (device_id) WHERE scope = 'device';
CREATE INDEX alert_rules_type ON alert_rules (classroom_id, device_type) WHERE scope = 'type';
