-- What SAM must hold about each user: whether they are a minor and their
-- guardian's consent, and what SAM last confirmed it holds, so the user
-- applier asserts a value only when it differs
-- (docs/design/sam-integration.md#who-acts-in-sam). And a mark on outbox
-- changes that take access away, so a held one stands out: until it is
-- delivered, SAM is more permissive than this service.

-- minor is null until onboarding says; SAM treats a user with no minor
-- assertion as a minor. consent is this service's state; only given and
-- withdrawn are asserted to SAM.
ALTER TABLE users
    ADD COLUMN minor       boolean,
    ADD COLUMN consent     text CHECK (consent IN ('pending', 'given', 'withdrawn', 'not_needed')),
    ADD COLUMN sam_minor   boolean,
    ADD COLUMN sam_consent text CHECK (sam_consent IN ('given', 'withdrawn'));

ALTER TABLE sam_outbox ADD COLUMN takes_access_away boolean NOT NULL DEFAULT false;
