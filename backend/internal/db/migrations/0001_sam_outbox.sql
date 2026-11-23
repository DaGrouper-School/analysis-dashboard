-- Changes SAM must reflect, written in the same transaction as the change
-- and sent by one worker, so SAM converges without any request waiting on
-- it (docs/design/sam-integration.md#the-outbox).

-- The last version given to each entity. Taking the next one locks the
-- entity's row, so two transactions changing one entity queue in order.
CREATE TABLE sam_outbox_versions (
    entity  text PRIMARY KEY,
    version bigint NOT NULL
);

CREATE TABLE sam_outbox (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- What the row is about, such as user:<id>.
    entity          text NOT NULL,
    -- Which applier sends it.
    kind            text NOT NULL,
    version         bigint NOT NULL,
    -- The entity's whole desired state, not a delta.
    payload         jsonb NOT NULL,
    -- The account it is sent as; null for this service.
    actor_id        uuid,
    -- pending until sent; done once SAM has it; held for an operator;
    -- superseded when a later version was sent in its place.
    state           text NOT NULL DEFAULT 'pending'
                    CHECK (state IN ('pending', 'done', 'held', 'superseded')),
    attempts        integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL,
    -- When the first attempt went out: SAM replays an idempotency key's
    -- answer only for a while after it.
    first_sent_at   timestamptz,
    last_error      text,
    created_at      timestamptz NOT NULL,
    finished_at     timestamptz,
    UNIQUE (entity, version)
);

-- The worker's pass: the latest pending version of each entity due.
CREATE INDEX sam_outbox_pending ON sam_outbox (entity, version DESC) WHERE state = 'pending';
CREATE INDEX sam_outbox_held ON sam_outbox (entity) WHERE state = 'held';
