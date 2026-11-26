-- Platform admins in SAM: one group at the tenant root, and who SAM holds
-- in it (docs/design/accounts-and-sign-in.md#platform-admins).

-- Groups this service keeps at the tenant root, by name, once SAM confirms
-- them. So far, the platform admins' group.
CREATE TABLE tenant_groups (
    name       text PRIMARY KEY,
    group_id   uuid NOT NULL UNIQUE,
    created_at timestamptz NOT NULL
);

-- Whether SAM holds the user in the admins' group, once it confirms it.
ALTER TABLE users ADD COLUMN sam_admin boolean NOT NULL DEFAULT false;
