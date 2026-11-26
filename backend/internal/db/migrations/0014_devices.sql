-- Devices: SAM devices in their lab's space, registered and retired as
-- the person who did it (docs/design/labs-and-readings.md#devices).

-- A device, its display name and hardware identifier kept here, and SAM's
-- device and its version once SAM confirms them. A removed device keeps
-- its row, and SAM its readings.
CREATE TABLE devices (
    id            uuid PRIMARY KEY,
    lab_id        uuid NOT NULL REFERENCES labs ON DELETE CASCADE,
    classroom_id  uuid NOT NULL REFERENCES classrooms ON DELETE CASCADE,
    name          text NOT NULL,
    -- The registry device type, by SAM's ID.
    type          text NOT NULL,
    identifier    text NOT NULL,
    registered_by uuid REFERENCES users ON DELETE SET NULL,
    registered_at timestamptz NOT NULL,
    removed_by    uuid REFERENCES users ON DELETE SET NULL,
    removed_at    timestamptz,
    sam_device_id uuid UNIQUE,
    sam_version   integer
);
-- An identifier is unique among a classroom's live devices.
CREATE UNIQUE INDEX devices_live_identifier ON devices (classroom_id, identifier) WHERE removed_at IS NULL;
CREATE INDEX devices_lab ON devices (lab_id) WHERE removed_at IS NULL;
