-- The guardian consent text, in versions: what a guardian agrees to, kept
-- here so every consent names the words it was given against
-- (docs/design/classrooms-and-permissions.md#guardian-consent).

-- One row per version and language: the sheet's parts, as the front end's
-- ConsentSheetContent, with {classroom}, {institution} and {decimals} where
-- each classroom's details go. A version is published in every language at
-- once; the latest is current.
CREATE TABLE consent_texts (
    version           integer NOT NULL CHECK (version > 0),
    language          text NOT NULL,
    content           jsonb NOT NULL,
    published_at      timestamptz NOT NULL,
    published_by      uuid REFERENCES users ON DELETE SET NULL,
    published_by_name text NOT NULL,
    PRIMARY KEY (version, language)
);

-- A published version is never changed or deleted: consents name it. The
-- one change allowed is forgetting the publisher's account when it is
-- deleted; their name at the time stays.
CREATE FUNCTION consent_texts_published() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND NEW.published_by IS NULL
        AND (NEW.version, NEW.language, NEW.content, NEW.published_at, NEW.published_by_name)
            IS NOT DISTINCT FROM (OLD.version, OLD.language, OLD.content, OLD.published_at, OLD.published_by_name) THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'a published consent text is never changed or deleted'
        USING ERRCODE = 'restrict_violation';
END
$$;

CREATE TRIGGER consent_texts_never_changed BEFORE UPDATE OR DELETE ON consent_texts
    FOR EACH ROW EXECUTE FUNCTION consent_texts_published();
CREATE TRIGGER consent_texts_never_truncated BEFORE TRUNCATE ON consent_texts
    FOR EACH STATEMENT EXECUTE FUNCTION consent_texts_published();
