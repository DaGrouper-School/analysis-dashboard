# Operations

Running the service once it is deployed. Configuration is in [Development](development.md#configuration).

## Phone codes

- **The SMS provider.** Development and test write codes to the log. No other environment does: until a deployment configures an SMS provider, phone sign-in refuses to send a code, and the error is logged.
- **The overall send limit.** `DGS_SMS_SENDS_PER_HOUR` (200 by default) caps the codes sent per hour across every number and address, which caps what SMS pumping can cost. Set it from the expected sign-up rate. Past it, people asking for a code get `429` with `Retry-After` until the oldest send of the hour leaves the window.
- **The client address behind a proxy.** The per-address limits key on the client's address. Behind a load balancer or proxy that is the proxy's own, so set `DGS_CLIENT_IP_HEADER` to the header it puts the client's address in, such as `X-Forwarded-For`: the service takes the last address in it, the one the proxy added. Set it only behind a proxy that sets the header itself, or a client could name any address.

## Email

Email goes through the SMTP server in `DGS_SMTP_*` ([Configuration](development.md#configuration)); any mail service that offers SMTP will do. Outside development and test the server is required, and the connection is encrypted: STARTTLS on 587 by default, or TLS from the start with `DGS_SMTP_SECURITY=tls`, usually on 465. The server's certificate is checked against the system's roots.

- **Checking the settings.** `dgs email test ADDRESS` reads the same configuration as `dgs serve` and sends a test email to `ADDRESS`. An error names the step that failed: connecting, TLS, signing in or sending. It needs no database.
- **The sender.** `DGS_SMTP_FROM` is the sender on every email. Its domain also ends each Message-ID; publish SPF and DKIM for it with the mail service, or mail may land as spam.
- **In development.** Without `DGS_SMTP_HOST`, email is written to the log. To see it as email, point `DGS_SMTP_*` at a local catcher such as Mailpit with `DGS_SMTP_SECURITY=none`.

## Platform configuration

Platform admins keep the stages, regions, host questions and settings ([Platform configuration](development.md#platform-configuration)). Pages read them on each request, so a change shows at once, with no restart.

```sh
dgs seed                                  # once on a new deployment; safe to run again
dgs settings                              # the settings, and whether the seed set each
dgs settings get privacy > privacy.json   # edit, then
dgs settings put privacy privacy.json     # - reads standard input
```

- **The seed.** `dgs seed` loads the configuration and the registry's overlay built into the binary. It is safe to run in every environment and on every release: it adds a list's rows only while the list is empty, updates what it wrote last to the running release's seed, and keeps whatever an admin or operator has changed. It prints what it did with each setting and row.
- **Checks.** `dgs settings put` refuses a value that is not the setting's shape or breaks one of its rules, and lists every problem with where it is. Nothing is stored then.
- **The log.** Every change is logged with who made it: an admin by name, `operator` for `dgs settings put`, `seed` for the seed.
- **Back to the seed.** A setting someone changed stays theirs. To take the seed's value again, put it from the release's `internal/settings/seed.json`, such as `jq .privacy internal/settings/seed.json | dgs settings put privacy -`.

### Publishing a consent text

The guardian consent text is published in versions, every language at once, from the admin page or from a terminal. A version is never changed: a correction is a new version. Consents name the version they were given against, so a guardian who opened their link before a new version reloads the new words.

```sh
dgs consent publish consent.json   # the next version; - reads standard input
```

The file maps each language code to the sheet's parts, as `internal/classrooms/consent.json` does: `title`, `intro`, `sections` (each a `heading` and `body`), `choices` (`give` and `withdraw`), `contributorField`, `fields` and `officeNote`. `{classroom}`, `{institution}` and `{decimals}` are filled in for each classroom. A version keeps every language of the one before and always has `en`. A file that breaks a rule is refused with every problem named, and nothing is published.

### Configuring a classroom

Platform admins set a classroom's stage, region and profile fields from the admin page. The same change from a terminal:

```sh
dgs classroom configure DGC-HG2W5K classroom.json
```

The file holds any of `stageId`, `regionId` (`null` clears it) and `profile`. A field left out stays as it is, and each profile field replaces only its own value: `areaCode`, `language`, `description`, `ecosystems`, `hostAgreementSigned`, `projects`, `sessions`, `custodianRole`, and `institution` with `history`, `hub` and `website`. A stage or region must already exist. A file that breaks a rule is refused with every problem named, and nothing changes.

## Platform admins

Platform admins are named by an operator, never from the dashboard. The person signs in once, then:

```sh
dgs admins                                 # who is an admin, and whether SAM holds them as one yet
dgs admins add amina@dagrouper.example     # by their Google email, or their account ID
dgs admins remove amina@dagrouper.example
```

Removing takes effect here at once: they lose the admin page and every admin action on their next request. SAM follows through the outbox, and until it does, `dgs outbox held` lists a change that cannot reach SAM. Each change is logged in `configuration_changes` as the operator's. A person who signed in only with their phone is named by their account ID.

## Places

`DGS_PLACES_API_KEY` is a Google Places key with the Places API enabled. Restrict it to that API and to the service's outbound addresses: it is sent to Google from the server only, in a request header. Each signed-in person may look up 300 places an hour; autocomplete asks as they type, so this covers filling in a few addresses and stops a script.

## The media store

Photos are kept in an S3-compatible object store, `DGS_MEDIA_URL` with the key in `DGS_MEDIA_ACCESS_KEY` and `DGS_MEDIA_SECRET_KEY`. The compose file runs SeaweedFS's S3 gateway as `media`, with the development key in `media-s3.json`; `make test-media` tests the store against `test-media`.

- **The bucket.** `DGS_MEDIA_BUCKET` (`dgs-media`) is made at start-up when it has none, so the key needs the right to make buckets, or the bucket is made beforehand. The service does not start while the store cannot be reached.
- **Names.** Each file is named by the SHA-256 of its bytes, with its type's extension, and the `media` table records its type, size and what it belongs to. A file is written once and never changed, so the store can keep versions off.
- **Serving.** Files are served at `/api/media/{name}` from the store, with the type the table records and a year's cache: a name never changes what it holds. A file the table records that the store lacks answers 404 and logs an error.
- **Back-ups.** Back the bucket up with the database: a restored database naming files the bucket lacks shows no photos for them.

## Signing keys

User tokens are signed with Ed25519 keys from the key file `DGS_SIGNING_KEYS_FILE`. Outside development and test, start-up is refused without one. Keep it as a secret, readable only by the service.

```json
{"keys": [
  {"kid": "20261123-3f9a1c", "seed": "<32-byte seed, unpadded base64url>", "signs": true}
]}
```

- Exactly one key signs. Every key in the file is published in the key set at `/api/.well-known/jwks.json`, so SAM verifies a token signed by any of them.
- `dgs keys new` prints a new key, not yet signing, named by the day and a random suffix.
- Every instance reads the file at start-up, so a change takes effect as each one restarts.

### Rotating

SAM keeps a fetched key set for five minutes. A key is therefore published before it signs, and stays published after:

1. Add the key `dgs keys new` prints, with `signs` false, and restart every instance. The set now publishes it.
2. At least five minutes later, move `signs: true` to the new key and restart every instance. SAM's cached set already holds the new key, and tokens signed by the old one still verify.
3. A day later, remove the old key and restart.

### A lost or leaked key

Tokens last five minutes, so a leaked key is useful only while it is published. Add a new key and make it sign at once, removing the leaked one in the same change, then restart every instance. SAM refetches the set when a token names a key it does not hold, at most once every thirty seconds, so calls made as a person may be refused for up to half a minute.

## Held changes

Changes reach SAM through the outbox. A change SAM refuses, whose actor's account is gone, or that this version cannot read is held, and logged at warning level as `outbox change held`. Later changes to the same entity are still sent, and one that reaches SAM supersedes the held one.

```sh
dgs outbox held                # what is held: the entity, kind, version, who it is sent as, attempts and the error
dgs outbox release ENTITY      # send the entity's latest held change again on the worker's next pass
```

Release a change once what refused it is fixed, for example once the person has the rights SAM asked for. A released change that SAM refuses again is held again.

**Changes that take access away come first.** A held change after which SAM should allow less, such as a guardian's withdrawal or a member's removal, is logged at error level as `outbox change held that takes access away`, and `dgs outbox held` lists it first, with `YES` under `TAKES ACCESS AWAY`. Until it is delivered SAM still allows what this service has taken away. Nobody reaches SAM except through this service, which checks its own record first, but resolve these before the rest.

### Releases and data terms SAM refuses

A release is `release:<classroom id>`, a publish sent as the person releasing, and a classroom's terms are `policy:<classroom id>`, sent as the person who changed them. Each waits, never held, until SAM holds the person's rights. SAM refuses one, and it is held, when the person may no longer do it, for example a facilitator releasing after the head took release back. The cutoff here has already moved, so visitors' pages read as released what SAM has not published yet: have the head release through a later time, which queues a new publish covering it, or release the held one once the person has the right again.

```sh
dgs outbox held                          # lists release:… and policy:… with the error
dgs outbox release release:<classroom id> # publish it again
```

A publish names its release in its idempotency key, so sending it again publishes nothing twice.

### Registry entries SAM refuses

Domains, values, device types and templates platform admins add are this tenant's own SAM entries, sent through the outbox as `registry:<kind>:<id>`, such as `registry:parameter:dgs:soil_ec`. The admin page shows an entry as soon as it is made, so until SAM has it, devices cannot yet send it. SAM refuses an entry with:

- `invalid_request`: SAM's own rules on the definition, such as a name over 200 characters;
- `mapping_conflict`: keys, fields or derivations that name the entry could not use it as changed;
- `parameter_in_use` or `device_type_in_use`: readings are stored, so a unit, kind or key's mapping is fixed.

The admin who made the change is in the registry's change log. Have them put the entry back as SAM holds it, or add a new value, then release the entity. An entry waiting on another, such as a device type on its values, is retried, never held, until that one reaches SAM.

## After a database restore

A database restored from a backup has forgotten what happened after the backup was taken, but SAM has not. Users who signed up after the backup are gone from this database, yet SAM still holds their principals under their participant IDs. At start-up the restore guard moves the participant ID sequence past every ID SAM knows, and logs `participant ID guard moved the sequence`. New users then get IDs SAM has never seen.

A registration can still be held after a restore, with SAM answering that a principal already exists under the user's participant ID. `dgs outbox held` lists it as `user:<user id>`, with an error saying so. Which of two cases it is decides what to do, and the service cannot tell them apart, so it never adopts the principal itself:

| The user | SAM's principal is | Do |
|---|---|---|
| Signed up after the restore, before the guard ran, such as while SAM could not be asked at start-up | Someone else's: a user the restore lost, who had this ID | `dgs accounts renumber USER` |
| Was in the backup, but their registration reached SAM after the backup was taken. Its answer is lost with the restore, and SAM no longer replays it after a day | Their own | `dgs accounts adopt USER` |

To tell them apart, compare when the user was created here (`SELECT created_at FROM users WHERE id = '<user id>'`) with when the backup was taken. A user created before the backup is the second case; one created after the restore is the first. `dgs accounts adopt` prints when SAM registered the principal, which falls after the backup in the second case.

- **Renumbering** gives the user the next free participant ID and queues their registration under it.
- **Adopting** records SAM's principal as the user's and completes their held registration.

Both refuse a user whose principal is already recorded. Never adopt a principal you have not checked: adopting someone else's lets one person act as another in SAM, with their grants and their data.

## The tenant file

`DGS_SAM_TENANT_FILE` is what `sam tenant create -json` printed when this service was onboarded, and holds its service key, shown that once. Keep it as a secret. Onboarding again for an onboarded tenant prints no key: a lost service key is replaced with an emergency key issued by SAM's operator ([SAM's tenants](https://github.com/DaGrouper-School/sam/blob/main/docs/tenants.md)).
