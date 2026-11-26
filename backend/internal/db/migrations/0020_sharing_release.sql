-- A classroom's release cutoff, which never moves back, and the version
-- of its data policy SAM holds once SAM confirms it
-- (docs/design/dashboards-and-public-site.md#sharing-and-release).
ALTER TABLE classrooms
    ADD COLUMN released_through   timestamptz,
    ADD COLUMN sam_policy_version integer;
