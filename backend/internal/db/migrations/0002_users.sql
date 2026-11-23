-- Accounts, each with the participant ID that is its SAM principal's
-- subject and the only identifier of a person SAM holds
-- (docs/design/sam-integration.md#participant-ids-and-the-restore-guard).
-- The rest of an account arrives with sign-in and the profile.

-- Participant IDs are given in order and never reused: the sequence never
-- cycles, and the restore guard only ever moves it forward.
CREATE SEQUENCE participant_ids AS bigint START 1;

CREATE TABLE users (
    id               uuid PRIMARY KEY,
    participant_id   bigint NOT NULL UNIQUE DEFAULT nextval('participant_ids'),
    -- The account's SAM principal, once registered.
    sam_principal_id uuid UNIQUE,
    created_at       timestamptz NOT NULL
);

ALTER SEQUENCE participant_ids OWNED BY users.participant_id;
