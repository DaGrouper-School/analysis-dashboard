-- Invitations to a classroom, and membership as SAM holds it
-- (docs/design/classrooms-and-permissions.md#invitations).

-- An invitation by email to a role. Only the token's hash is kept. One
-- from a platform admin with no role in the classroom, or the head's when
-- a classroom is created, is sent as this service (by_service); any other
-- names its inviter, and is refused once they are gone.
CREATE TABLE invitations (
    id           uuid PRIMARY KEY,
    classroom_id uuid NOT NULL REFERENCES classrooms ON DELETE CASCADE,
    email        text NOT NULL,
    role         text NOT NULL CHECK (role IN ('institution_head', 'facilitator', 'community_scientist', 'researcher')),
    token_hash   bytea NOT NULL UNIQUE,
    invited_by   uuid REFERENCES users ON DELETE SET NULL,
    invited_by_name text NOT NULL,
    by_service   boolean NOT NULL,
    status       text NOT NULL CHECK (status IN ('pending', 'accepted', 'revoked')),
    created_at   timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL,
    decided_at   timestamptz
);
CREATE INDEX invitations_email ON invitations (email) WHERE status = 'pending';
CREATE INDEX invitations_inviter ON invitations (invited_by) WHERE status = 'pending';

-- The role SAM holds a member in, once it confirms it, and whether they
-- have left here. A member who left keeps their row until SAM has removed
-- them, and reads skip it.
ALTER TABLE classroom_members
    ADD COLUMN sam_role text CHECK (sam_role IN ('institution_head', 'facilitator', 'community_scientist', 'researcher')),
    ADD COLUMN removed boolean NOT NULL DEFAULT false;
