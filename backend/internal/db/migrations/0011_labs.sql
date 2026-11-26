-- Labs: the places a classroom measures, kept here and created in SAM as
-- a space under the classroom's, with a site at the lab
-- (docs/design/labs-and-readings.md#labs).

-- A lab, numbered in its classroom from 1: its site's code is the
-- classroom's code and this number, such as DGC-HG2W5K-01. Its space and
-- the version of its site are recorded once SAM confirms them.
CREATE TABLE labs (
    id           uuid PRIMARY KEY,
    classroom_id uuid NOT NULL REFERENCES classrooms ON DELETE CASCADE,
    number       integer NOT NULL CHECK (number > 0),
    name         text NOT NULL,
    place_name   text NOT NULL,
    country      text NOT NULL,
    lat          double precision NOT NULL CHECK (lat BETWEEN -90 AND 90),
    lng          double precision NOT NULL CHECK (lng BETWEEN -180 AND 180),
    -- The front end's SiteProfile: the questionnaire and the classroom's
    -- notes on each value.
    site_profile jsonb NOT NULL,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    space_id     uuid UNIQUE,
    site_version integer,
    UNIQUE (classroom_id, number)
);
