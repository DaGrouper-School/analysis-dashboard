-- Global Classrooms, kept here and created in SAM, and who is a member of
-- each (docs/design/classrooms-and-permissions.md).

-- A classroom: its code, which never changes, what admins configure, its
-- sharing settings and data terms, and its SAM space and four role groups
-- once SAM confirms them. A stage or region a classroom is at cannot be
-- deleted.
CREATE TABLE classrooms (
    id                   uuid PRIMARY KEY,
    code                 text NOT NULL UNIQUE,
    name                 text NOT NULL,
    institution_name     text NOT NULL,
    setup                text NOT NULL CHECK (setup IN ('self_service', 'admin_assisted')),
    stage_id             text NOT NULL REFERENCES stages,
    region_id            text REFERENCES regions,
    -- The front end's ClassroomSharing and DataTerms.
    sharing              jsonb NOT NULL,
    terms                jsonb NOT NULL,
    -- What the classroom says about itself: the front end's ClassroomProfile.
    profile              jsonb NOT NULL,
    -- The head invited when it was created, until their invitation is sent.
    head_email           text,
    created_by           uuid REFERENCES users ON DELETE SET NULL,
    created_at           timestamptz NOT NULL,
    updated_at           timestamptz NOT NULL,
    space_id             uuid UNIQUE,
    heads_group          uuid,
    facilitators_group   uuid,
    scientists_group     uuid,
    researchers_group    uuid
);

-- A member's role in a classroom: their membership of its role group.
CREATE TABLE classroom_members (
    classroom_id uuid NOT NULL REFERENCES classrooms ON DELETE CASCADE,
    user_id      uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    role         text NOT NULL CHECK (role IN ('institution_head', 'facilitator', 'community_scientist', 'researcher')),
    joined_at    timestamptz NOT NULL,
    PRIMARY KEY (classroom_id, user_id)
);
CREATE INDEX classroom_members_user ON classroom_members (user_id);
