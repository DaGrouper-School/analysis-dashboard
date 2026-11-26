-- Stored files, such as institution photos: each named by its content
-- hash, so a name never changes what it holds, with what it belongs to
-- (docs/design/dashboards-and-public-site.md#photos).
CREATE TABLE media (
    name         text PRIMARY KEY,
    content_type text NOT NULL,
    size         integer NOT NULL CHECK (size > 0),
    kind         text NOT NULL,
    owner_id     uuid NOT NULL,
    created_at   timestamptz NOT NULL
);

CREATE INDEX media_owner ON media (kind, owner_id);
