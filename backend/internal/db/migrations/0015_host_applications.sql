-- Applications to host a Global Classroom, made without signing in. Only
-- the status link's token's hash is kept. An accepted application names
-- the classroom created from it, once there is one.
CREATE TABLE host_applications (
    id               uuid PRIMARY KEY,
    institution_name text NOT NULL,
    contact_name     text NOT NULL,
    contact_role     text NOT NULL,
    contact_email    text NOT NULL,
    -- E.164, or empty.
    contact_phone    text NOT NULL,
    place_name       text NOT NULL,
    country          text NOT NULL,
    ecosystems       text[] NOT NULL,
    learners         text NOT NULL,
    study_goals      text NOT NULL,
    token_hash       bytea NOT NULL UNIQUE,
    status           text NOT NULL CHECK (status IN ('submitted', 'accepted', 'declined')),
    submitted_at     timestamptz NOT NULL,
    decided_at       timestamptz,
    decided_by       uuid REFERENCES users ON DELETE SET NULL,
    classroom_id     uuid UNIQUE REFERENCES classrooms ON DELETE SET NULL,
    CHECK ((status = 'submitted') = (decided_at IS NULL)),
    CHECK (classroom_id IS NULL OR status = 'accepted')
);

CREATE INDEX host_applications_newest ON host_applications (submitted_at DESC, id);

-- An application's answers, each to its question's row, which is retired
-- rather than deleted once answered.
CREATE TABLE host_answers (
    application_id uuid NOT NULL REFERENCES host_applications ON DELETE CASCADE,
    question_id    text NOT NULL REFERENCES host_questions,
    answer         text NOT NULL,
    PRIMARY KEY (application_id, question_id)
);
