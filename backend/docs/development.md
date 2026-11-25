# Development

How to build, run and test the service as it stands, and the conventions its code follows. So far the service has configuration, logging with request IDs, the HTTP conventions, its PostgreSQL database with migrations, calls to SAM as itself, as a person or as anonymous, the key set SAM verifies user tokens with, the outbox that delivers changes to SAM and the reconciler, the platform's configuration and its seed, `/healthz`, graceful shutdown, and integration tests against SAM and the API contract.

## Prerequisites

| Tool | Version | For |
|---|---|---|
| Go | 1.27 (`go.mod`) | Building and testing |
| golangci-lint | v2 (CI pins v2.15.0) | `make lint` |
| Docker with Compose | Recent | The databases, `make up`, the image |
| Node and npm | 22 | Regenerating and checking the API contract, from the repository root |
| GitHub CLI, signed in | Recent | Reading SAM's private module ([SAM's module](#sams-module)) |

## Running

```sh
make up                      # the service, its database and SAM, waiting until healthy
curl -s localhost:8090/healthz
make down                    # stops them and discards their data
```

`compose.yaml` runs the service in development, with `http://localhost:5173` as its front-end origin, published on `127.0.0.1:8090`, beside its databases and a SAM it is onboarded on:

| Service | Published on | For |
|---|---|---|
| `backend` | `127.0.0.1:8090` | This service |
| `db` | `127.0.0.1:5433`, database `dgs` | The development data, kept in the `dgs-db` volume until `make down` |
| `sam` | `127.0.0.1:8091` | SAM, built from the module version `go.mod` pins |
| `sam-init` | | Runs once before `sam`: sets the deployment up (`sam bootstrap -json`) and onboards this service (`sam tenant create -name dgs -registry`), keeping the operator's key (`operator.json`) and the tenant file (`tenant.json`) in the `sam-config` volume |
| `sam-db` | | SAM's database, on PostGIS, kept in the `sam-data` volume until `make down` |
| `test-db` | `127.0.0.1:55433` | The server the tests create their databases on, with PostGIS for the SAM the integration tests start. Its data is in memory, and it starts only with the `test` profile, which `make test` and `make db-up` use |

The ports differ from SAM's own stack, so both run side by side.

**SAM in compose.** The tenant was onboarded with issuer `http://backend:8090` and key set `http://backend:8090/api/.well-known/jwks.json`. SAM fetches the key set by container name, which `SAM_ALLOW_PRIVATE_KEY_SETS=true` allows, and never in production. In development the service signs with a key it generates at start-up, so a restart changes it; SAM fetches the set again when a token names a key it does not hold.

**Running on the host.** `make run` serves on `:8090` with `go run`, against the compose `db` and `sam`, which it starts, with the tenant file copied out to `.dev/tenant.json`. SAM still verifies user tokens against the compose backend's key set, so a call made as a person needs `make up`. Stop the compose `backend` first, since both listen on `8090`.

## Make targets

The targets are the same as SAM's. `make help` lists them. The Makefile sets `DGS_ENV=development`, `DGS_DATABASE_URL` to the compose `db` service and the `DGS_SAM_*` settings to the compose `sam`, unless the environment already sets them.

| Target | What it does |
|---|---|
| `help` | Lists the targets |
| `build` | Builds `bin/dgs` |
| `run` | Serves the API locally (`go run ./cmd/dgs serve`), starting the compose `db` and `sam` |
| `migrate` | Applies new migrations to `DGS_DATABASE_URL` (`go run ./cmd/dgs migrate`) |
| `test` | Runs every test with the race detector, against `DGS_TEST_DATABASE_URL` when it is set, otherwise against the compose `test-db`, which it starts |
| `test-short` | Runs the tests that need no database or SAM (`-short`) |
| `lint` | `go vet`, a `gofmt` check and `golangci-lint` |
| `tidy-check` | Fails when `go.mod` or `go.sum` is not tidy |
| `vulncheck` | Reports known vulnerabilities in dependencies (`govulncheck`) |
| `db-up` | Starts the development and test databases, and SAM |
| `up` | Builds and starts the service, its database and SAM, waiting until healthy |
| `down` | Stops the stack and discards its data, SAM's included |

## Configuration

Configuration comes from the environment (`internal/config`). Every problem is reported at once, and start-up stops on any.

| Variable | Default | Meaning |
|---|---|---|
| `DGS_ENV` | none: required (`make` sets `development`) | `development`, `test`, `staging` or `production`. Development conveniences exist only in the first two ([design](design/accounts-and-sign-in.md#environments)) |
| `DGS_LISTEN` | `:8090` | Listen address |
| `DGS_ALLOWED_ORIGINS` | `http://localhost:5173` in development and test; required elsewhere | Front-end origins allowed to call with credentials and to send writes, comma-separated. Each must be a bare origin such as `https://app.example`; a trailing `/` is dropped. Outside development and test each must use `https` |
| `DGS_LOG_FORMAT` | `text` in development and test, `json` elsewhere | `text` or `json` |
| `DGS_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| `DGS_DATABASE_URL` | none: required (`make` sets the compose `db`) | The PostgreSQL connection string |
| `DGS_DB_MAX_CONNS` | `10` | The most connections the pool opens, a whole number from 1 |
| `DGS_DB_STATEMENT_TIMEOUT` | `30s` | How long one statement may run before PostgreSQL cancels it, a Go duration of at least `1ms` |
| `DGS_SAM_URL` | none: required (`make` sets the compose `sam`) | SAM's URL. Outside development and test it must use `https` |
| `DGS_SAM_TENANT_FILE` | none: required | The tenant file: the JSON `sam tenant create -json` printed when this service was onboarded, with the service key ([SAM](#sam)) |
| `DGS_SAM_ISSUER` | none: required | The `iss` of user tokens: the `-issuer` this service was onboarded with |
| `DGS_SAM_AUDIENCE` | `sam` | The `aud` of user tokens: the `-audience` this service was onboarded with |
| `DGS_PUBLIC_URL` | `http://localhost:8090` in development and test; required elsewhere | The URL the service is reached at. The session cookie is `Secure` when it is `https`, which outside development and test it must be |
| `DGS_GOOGLE_CLIENT_ID` | none; required outside development and test | The Google OAuth client ID, the audience of the ID tokens sign-in accepts. Without it Google sign-in is refused |
| `DGS_SESSION_TTL` | `720h` (30 days) | How long an unused session lasts, a Go duration of at least `1h` |
| `DGS_DEMO` | `false` | `true` signs in the demo accounts by subject. Start-up is refused with it on outside development and test |
| `DGS_SMS_SENDS_PER_HOUR` | `200` | The most phone codes sent per hour overall, a whole number from 1, set from the expected sign-up rate ([Operations](operations.md#phone-codes)) |
| `DGS_CLIENT_IP_HEADER` | none | The header a trusted proxy puts the client's address in, such as `X-Forwarded-For`; the last address in it is the client's. Without it, the connection's address is |
| `DGS_PLACES_API_KEY` | none in development and test, where the offline list is used; required elsewhere | The Google Places key address lookup uses. It is sent to Google in a request header only, never to the browser |
| `DGS_SMTP_HOST` | none in development and test, where email goes to the log; required elsewhere | The SMTP server email is sent through ([Operations](operations.md#email)) |
| `DGS_SMTP_SECURITY` | `starttls` | `starttls` (STARTTLS required before anything is sent), `tls` (TLS from the start) or `none`, which only development and test allow, for a local catcher such as Mailpit |
| `DGS_SMTP_PORT` | `587` for `starttls`, `465` for `tls`, `25` for `none` | The SMTP server's port |
| `DGS_SMTP_USERNAME`, `DGS_SMTP_PASSWORD` | none: email is sent without signing in | The SMTP credentials, set together. They are sent with `AUTH PLAIN`, over an encrypted connection only, unless the server is this machine |
| `DGS_SMTP_FROM` | none; required with `DGS_SMTP_HOST` | The sender, such as `DaGrouper School <no-reply@dgs.example>` |
| `DGS_RECONCILE_INTERVAL` | `15m` | How often the reconciler compares SAM with this database, a Go duration of at least `1m` |
| `DGS_SIGNING_KEYS_FILE` | none in development and test, where a key is generated at start-up; required elsewhere | The key file user tokens are signed with ([Operations](operations.md#signing-keys)) |

Every variable the service reads is listed here. The tests also read `DGS_TEST_DATABASE_URL` ([Testing](#testing)).

## Package layout

| Package | Responsibility |
|---|---|
| `cmd/dgs` | The command. `dgs serve` (the default) applies new migrations, reads the tenant file and signing keys, then serves until `SIGINT` or `SIGTERM`, then stops taking requests and waits up to ten seconds for those in flight. `dgs migrate` applies new migrations and exits. `dgs keys new` prints a new signing key ([Operations](operations.md#signing-keys)). `dgs outbox held` and `dgs outbox release ENTITY` list and release changes held for an operator ([Operations](operations.md#held-changes)). `dgs accounts renumber USER` and `dgs accounts adopt USER` resolve a registration held after a database restore ([Operations](operations.md#after-a-database-restore)). `dgs email test ADDRESS` sends a test email ([Operations](operations.md#email)). `dgs settings`, `dgs settings get KEY` and `dgs settings put KEY FILE` list, read and set the platform's settings, and `dgs seed` loads the platform's configuration ([Operations](operations.md#platform-configuration)) |
| `internal/config` | Reads and checks the configuration |
| `internal/logx` | Sets up `slog` in text or JSON; carries the request ID so every log line written while serving a request names it |
| `internal/httpx` | The error format, `WriteJSON`, `WriteError`, the `Handler` type, strict decoding |
| `internal/db` | Opens the pool, applies the migrations, transactions (`InTx`) and advisory locks |
| `internal/accounts` | Accounts and their participant IDs; registering each account's SAM principal through the outbox; the restore guard |
| `internal/notify` | Delivers messages: the SMS and email interfaces, the SMTP provider, the log providers for development and test, and a recorder of texts for tests |
| `internal/places` | The places module: address autocomplete and lookup through a provider, and the calling codes |
| `internal/settings` | The platform's settings, their checks and seed, the configuration change log, and seeding a list |
| `internal/classrooms` | The classrooms module. So far, the stages and regions and their seed |
| `internal/hosting` | The hosting module. So far, the host application's questions and their seed |
| `internal/jsoncheck` | Strict decoding and checks of configuration, reporting every problem at once |
| `internal/samaccess` | Calls SAM as this service, as a person through a user token or as anonymous; the signing keys and the key set; SAM's errors translated; the outbox, its worker and the reconciler |
| `internal/server` | `server.New(Deps)`, the `Module` interface, the middleware, `/healthz` and the API's not-found answer |
| `internal/testutil` | A fresh database per test (test code only) |
| `internal/contract` | Reads `openapi.yaml` and checks responses against it (test code only) |
| `internal/testenv` | Runs SAM built from source, with this service onboarded (test code only) |
| `internal/apitest` | The integration harness and its tests (test code only) |

The areas the code is organised into, and the data each owns, are in the design overview ([Data ownership](design/README.md#data-ownership)).

## The HTTP layer

Every request passes through, in order:

1. **Request ID.** A random ID, in the request's context and in the `X-Request-ID` response header.
2. **Request log.** One line per request with method, path, status and duration in milliseconds, except `/healthz`.
3. **CORS.** For an `Origin` in `DGS_ALLOWED_ORIGINS`: `Access-Control-Allow-Origin` with credentials and `Vary: Origin`. A preflight is answered `204` with methods `GET, POST, PUT, PATCH, DELETE`, header `Content-Type` and a ten-minute max age.
4. **Cross-site write check.** A request other than `GET`, `HEAD` or `OPTIONS` whose `Origin` is neither this service's own nor in `DGS_ALLOWED_ORIGINS` is refused with `403 forbidden`.
5. **Routes.** `GET /healthz` answers `200 {"status":"ok"}` while the database answers a ping within two seconds, and `503 {"status":"unavailable"}` otherwise. Modules mount their routes under `/api`. Any other path under `/api/` answers `404 not_found`.

Server timeouts: ten seconds for headers, sixty for the whole request, two minutes for an idle keep-alive connection.

### Errors

Every error is JSON `{code, message, fields}`. `code` is one of the front end's `ServiceErrorCode` values (`frontend/src/lib/services/types.ts`), `message` is shown to the user as it is, and `fields` holds a message per input.

| Code | Status | Made by |
|---|---|---|
| `unauthenticated` | 401 | `httpx.Unauthenticated()` |
| `invalid_input` | 400 | `httpx.Invalid(fields)`, `httpx.CheckFields(fields)`, `httpx.FieldError(code, field, message)` |
| `code_invalid`, `code_expired` | 400 | `httpx.FieldError` |
| `not_found` | 404 | `httpx.NotFound(message)` |
| `conflict` | 409 | `httpx.New(httpx.CodeConflict, message)` |
| `forbidden` | 403 | `httpx.Forbidden(message)` |
| `rate_limited` | 429 | `httpx.RateLimited(message, retryAfter)`, which sets `Retry-After` |
| `internal` | 500 | Any error that is not an `*httpx.Error`: logged with the request ID, shown as a generic message |

## SAM

`internal/samaccess` is the only package that calls SAM, and reaches modules through `server.Deps.SAM` ([SAM integration](design/sam-integration.md)).

| Calling as | With | For |
|---|---|---|
| This service | `Service()`: the service key from the tenant file | Changes made by policy; checks before a change |
| A person | `User(Person)`: the service key, and a user token for them in `SAM-Act-As` | Reads as the viewer; changes a person makes; relayed submissions |
| Anonymous | `Anonymous()` | Public pages |

- **User tokens** are EdDSA JWTs signed with the signing key: `kid`, `iss` (`DGS_SAM_ISSUER`), `aud` (`DGS_SAM_AUDIENCE`), `sub` (the participant ID), `azp` (this service's name in SAM, from the tenant file), and `iat` and `exp` five minutes apart. A token is signed for each call.
- **The key set** is served at `GET /api/.well-known/jwks.json`, outside the front end's contract, with every key in the key file, cached for five minutes.
- **A person with no SAM principal yet** gets `samaccess.ErrUnregistered` from `User`, without a call to SAM, and is answered as a person with no grants.
- **Errors.** `samaccess.Translate(ctx, err)` turns a SAM error into the front end's. SAM's detail is logged at info, never shown:

| SAM answers | The front end gets |
|---|---|
| `400` or `422` (`invalid_request` and an endpoint's own) | `invalid_input` |
| `401 unauthenticated` | `forbidden`, logged at error as a fault in this service's credentials |
| `403` | `forbidden` |
| `404` or `410` | `not_found` |
| `409` or `412` (`already_exists`, `version_mismatch` and the like) | `conflict` |
| `429` | `rate_limited`, keeping `Retry-After` |
| `5xx`, a timeout, SAM unreachable, or a call SAM refuses as malformed (`idempotency_key_reused`, `method_not_allowed`, `body_too_large`) | `internal`, logged with the request ID |

Calls to SAM time out after fifteen seconds. `/healthz` does not ask SAM: this service answers while SAM is down.

### The outbox

A change SAM must reflect is never sent from a handler. The service function that makes it writes the entity's whole desired state to the outbox, in the same transaction, and the worker sends it ([The outbox](design/sam-integration.md#the-outbox)).

- **Enqueueing.** `samaccess.Enqueue(ctx, tx, now, Change{Entity, Kind, Payload, Actor})`. `Entity` names what the change is about, such as `user:<user id>`, and never a participant ID or a row ID a restored database could give out again. `Actor` is the account the change is sent as, or nil for this service. `TakesAccessAway` marks a change after which SAM should allow less than before, such as consent withdrawn or a member removed: until it is delivered SAM is more permissive than this service, so held, it is logged at error level and `dgs outbox held` lists it first.
- **Kinds.** A module whose changes reach SAM declares its kinds with `OutboxKinds() []samaccess.Kind`: a name, whether it is sent as a person (`ByPerson`), and `Apply(ctx, Attempt)`, which sends the desired state through `Attempt.SAM`, the client acting as the change's actor.
- **Idempotency keys.** Every write `Apply` sends carries `Attempt.WithKey(ctx, what)`, the key `<entity>:<what>:v<version>`, so a retry after a lost answer gets the first answer back. When `Attempt.Replayed` is false, the first attempt is more than a day old and SAM no longer replays it, so a change that must happen once checks what SAM holds before sending again.
- **What Apply returns.** `nil` once SAM holds the change. `samaccess.ErrNotYet` while another entity must reach SAM first: the change is retried. `samaccess.Hold(err)` for what retrying cannot fix. SAM's own answers are sorted by the worker: `unavailable`, `internal`, `rate_limited` (after `Retry-After`), `idempotency_in_progress`, any 5xx and timeouts are retried, with backoff from a second up to five minutes; `forbidden`, `unauthenticated`, `invalid_request`, `attenuation_violation`, `idempotency_key_reused` and any other refusal hold the change.
- **The worker** runs on the instance holding the outbox lock, checks every second, and sends only the latest pending version of each entity. Once SAM holds it, the entity's older versions, waiting or held, are superseded. A change sent as a person waits until their principal is registered, and is held if their account is gone or the kind and the change disagree about who acts. Accounts are found through the module implementing `samaccess.Actors`.
- **The reconciler** runs every `DGS_RECONCILE_INTERVAL` on the instance holding the reconciler lock, and runs each `samaccess.Check` that modules declare with `Checks()`. A check compares one kind of entity in SAM with this database, a page at a time, and enqueues a correction where they differ. A failing check is logged and runs again on the next pass.

Both stop when the service does, before the database closes.

### Signing in

Sign-in is the accounts module's (`internal/accounts`), as are the tables it keeps ([Accounts and sign-in](design/accounts-and-sign-in.md)).

- **Google.** `POST /api/auth/google` takes an ID token from Google Identity Services and verifies its signature against Google's keys, the issuer, the audience (`DGS_GOOGLE_CLIENT_ID`) and the expiry, with thirty seconds' leeway. Google's keys are cached for an hour and fetched again for an unknown key at most once a minute; while Google cannot be reached, a cached key still verifies. A token that does not verify is `400 invalid_input` on `idToken`, and why is logged, never shown.
- **The first sign-in** creates the account, its outbox row, its Google identity (`google_identities`, by Google's subject) and the session in one transaction, and answers `isNewUser: true`. A new account whose verified email belongs to another account is `409 conflict`: the person signs in the way they did before. Nothing waits on SAM.
- **Sessions** are random tokens in the `session` cookie (`HttpOnly`, `SameSite=Lax`, `Secure` when `DGS_PUBLIC_URL` is `https`); `sessions` keeps only their SHA-256 hash. An unused session ends after `DGS_SESSION_TTL`; a use an hour or more after the last extension moves its end on and sends the cookie again. Ended sessions are removed every hour. `POST /api/auth/sign-out` deletes the session and clears the cookie.
- **Guarding a route.** Modules are given the module's `Sessions` and wrap a signed-in route in `RequireUser`, which answers `401 unauthenticated` without a live session and puts the user in the request's context, read with `accounts.CurrentUser`. `GET /api/auth/me` is guarded so, which the front end reads as signed out.
- **Phone codes.** `POST /api/auth/phone/start` normalises the number to E.164, as the front end's `normalizePhone` does, and texts a six-digit code through `notify`. `POST /api/auth/phone/verify` checks it and signs the holder in, creating their account on first sign-in, as Google sign-in does. The rules, all from the design's [Phone codes](design/accounts-and-sign-in.md#phone-codes):
  - **The code.** It comes from a secure random source and is stored only as its hash with the challenge's ID. It lives five minutes, is resent no sooner than thirty seconds, is used once, and a new one ends the last.
  - **Attempts.** A number has five while its codes live, and a new code keeps the count. One statement spends an attempt and reads the count before the code is compared, so parallel guesses spend the budget exactly.
  - **Limits.** Sends are limited per number (5 an hour), per client address (20) and overall (`DGS_SMS_SENDS_PER_HOUR`); verifications per number (10) and per address (50). They are counted over a sliding hour in `rate_events`, each key's `rate_limits` row locked while it is counted, so they hold across replicas. A refusal is `429 rate_limited` with `Retry-After`.
  - **The client address.** It is the connection's, or the last address in `DGS_CLIENT_IP_HEADER`. IPv6 addresses are keyed by their /64.
  - **Refusals.** A wrong code is `400 code_invalid`. An unknown, used, replaced or expired one is `400 code_expired`. A number without its country code is `400 invalid_input` on `phone`.
- **Demo sign-in.** With `DGS_DEMO` on, `GET /api/auth/demo-accounts` lists `demo_accounts`, and `POST /api/auth/google` takes a demo account's subject in place of a token. Otherwise the list is empty and a subject is no token. Demo mode also returns a phone code in the response as `devCode`, and does not text it.
- **A new account** is shown as no minor, with consent not needed, as the front end starts every account; SAM treats the person as a minor until onboarding asserts otherwise.

### The profile and onboarding

The profile is the accounts module's too: name, email, phone and address stay in this service, and SAM holds only the participant ID. Every call needs a session.

| Call | Does |
|---|---|
| `PATCH /api/me/profile` | Sets the name, at least two characters, and the email, stored lower-cased. A Google user's email stays verified while it is unchanged; any other typed in is not. An email another account holds verified is `409 conflict` |
| `POST /api/me/phone/start`, `POST /api/me/phone/verify` | Verify a phone with the codes and limits of phone sign-in, for the purpose `verify_phone`. A number another account has verified is `409 conflict`; a code sent to another account is `code_invalid` |
| `PUT /api/me/address` | Sets the address; a street, a city and a country at least |
| `POST /api/me/onboarding/complete` | Needs a name, an email, a verified phone and an address, and says what is still needed. For an adult it queues `minor=false` for SAM, which until then treats the person as a minor |
| `PUT /api/me/public-profile` | Shows or hides the name and role on contributor pages. Refused for a minor, `403 forbidden` |
| `POST /api/me/minor` | Marks a minor: consent pending, no public profile, `minor=true` queued for SAM. Marking again changes nothing |

### Places

`internal/places` looks addresses up for the profile and labs, so the provider's key stays on the server.

| Call | Does |
|---|---|
| `GET /api/places/autocomplete?q=…` | Suggestions for what the person typed, from two characters |
| `GET /api/places/{placeId}` | The address of a suggestion; `404` for a place the provider does not know |
| `GET /api/places/dial-codes` | The calling codes, pilot countries first. Needs no session |

- **Providers.** With `DGS_PLACES_API_KEY`, Google Places (the Places API): autocomplete, and a lookup asking only for the address's parts and location. Without it, the offline list, the same pilot-region places as the front end's mock; only development and test may run without a key.
- **Limits.** Autocomplete and lookups need a session, and each person has 300 an hour together, counted with the accounts module's limits; past it they get `429` with `Retry-After`.

### Platform configuration

The configuration platform admins keep ([Platform configuration](design/registry-and-content.md#platform-configuration)):

| What | Kept in | Module |
|---|---|---|
| Stages and regions | `stages`, `regions`: a row each, in display order | `internal/classrooms` |
| Host questions | `host_questions`: a row each, in order; a retired one has `retired_at` | `internal/hosting` |
| Settings | `platform_settings`: one value per key, `core_parameters`, `default_terms`, `privacy` | `internal/settings` |
| The change log | `configuration_changes`: who changed what, newest first | `internal/settings` |

- **Checks.** `settings.Check` decodes a setting strictly into its Go shape, the same as the front end's, with `internal/jsoncheck`: no unknown or missing field, no `null` where a value is required, no blank text, then the key's own rules, such as decimals from 0 to 4 or a rain reference that is one of the daily rain values. Every problem is reported at once, with where it is. A value is stored as it re-encodes from its shape, so the same value always stores the same way.
- **The log.** `settings.Record` writes a change to `configuration_changes`, with the admin's account and their name at the time, or `operator` or `seed`. Every module that keeps part of the configuration records its changes through it, in the same transaction as the change.
- **The seed.** Each module embeds its part of the seed: `internal/settings/seed.json`, `internal/classrooms/seed.json` and `internal/hosting/seed.json`, the same values as the front end's seed (`frontend/src/lib/data/seed.ts`). Change both together; a test checks each seed passes its checks. `dgs seed` runs them in one transaction under `db.SeedLock`. A list's rows are added only while the list is empty, so a row an admin deleted stays deleted; `settings.SeedList` holds that rule for every list. A setting or row is updated to the seed's while the seed wrote it last, which `seeded` records; anything else changed it, and the seed keeps it.

### Accounts in SAM

- **Participant IDs** come from the `participant_ids` sequence, in order, and are never reused. An account's ID is its SAM principal's subject, as a decimal string.
- **Registration.** `accounts.Create` makes the user and queues their `user` change in the same transaction. The worker registers the principal, as this service, under the key `user:<user id>:principal:v<version>`, and stores its ID. Until then the account is answered as a person with no grants.
- **Minor and consent.** `accounts.SetMinor` and `accounts.SetConsent` record them and queue the user, in the caller's transaction. The worker asserts `minor` (`true` or `false`) and `consent` (`given` or `withdrawn`) at the tenant root, as this service, each under its own key such as `user:<user id>:attr:consent:v3`. SAM's assertions are append-only, so the module keeps what SAM last confirmed and asserts only what differs. `minor` is asserted once onboarding knows it; until then SAM treats the user as a minor. `pending` and `not_needed` are this service's own states: SAM is told `withdrawn` only when consent it holds as `given` is no longer given here. Becoming a minor, or consent that was given ceasing to be, is queued as taking access away.
- **The reconciler** compares every user with SAM, a page of a hundred at a time: a user with no principal and nothing queued is queued; one whose `minor` or `consent` in SAM differs is corrected. A user it cannot compare is logged and compared on the next pass. SAM audits each read of a user's attributes.
- **The restore guard** runs at every start-up, in the background. It looks participant IDs up in SAM from the sequence's next one, doubling the step and then halving it, and moves the sequence past the last ID SAM knows, logging a warning when it does. Registrations wait until it has run, so while SAM cannot be asked the service serves and accounts wait to be registered.

## The database

PostgreSQL 17, through `pgx` (`internal/db`). `db.Open` opens a pool of at most `DGS_DB_MAX_CONNS` connections, each with `statement_timeout` set to `DGS_DB_STATEMENT_TIMEOUT`, and fails unless the database answers.

**Migrations** are SQL files in `internal/db/migrations`, embedded in the binary and applied in name order, each in its own transaction and recorded in `schema_migrations`. Every instance applies them on start, before it listens; an advisory lock makes the others wait while one applies them, and a failed migration stops start-up with nothing of it applied. The rules for writing one are in [the migrations folder](../internal/db/migrations/README.md).

**Transactions.** `db.InTx(ctx, q, fn)` runs `fn` in a transaction. When `ctx` already carries one (`db.WithTx`), `fn` runs in a savepoint of it, so a service function that needs a transaction works the same alone or inside a caller's.

**Advisory locks** are taken only through `db.Lock` (held by a connection until released), and `db.TryLock` and `db.LockTx` (held until the transaction ends), and every key is a constant in `internal/db/locks.go`, so no two uses share one by accident. `db.LockTx` waits for a lock held until the transaction ends. `MigrationLock` is held while migrations apply, `OutboxLock` for each pass of the outbox worker, `ReconcilerLock` for each pass of the reconciler, and `SeedLock` while `dgs seed` runs.

## Conventions

- **Modules.** A module implements `server.Module`: `Routes(mux *http.ServeMux, prefix string)`. It is built in `server.New` from `Deps` and added to the list its routes are mounted from. The command and tests both build the service through `server.New`. The full module pattern is in the [design overview](design/README.md#modules).
- **Handlers** are `httpx.Handler`: they return an error instead of writing one. Return an `*httpx.Error` for anything the user should see. A handler decodes the request, calls one service function and encodes the answer. Transactions, rules and outbox calls live in the module's service functions, not in handlers.
- **Request bodies** are read with `httpx.Decode`: JSON only, at most 1 MiB, unknown fields and trailing data refused. `httpx.DecodeLimit` allows a larger body for the few endpoints that need one.
- **Responses** are Go structs, never `map[string]any`. They are written with `httpx.WriteJSON`, which sets `Cache-Control: no-store` unless the handler set its own.
- **IDs** are `uuid.UUID` from the database to the handler, and strings only at the JSON boundary.
- **Dependencies** are passed in at construction, never set on a built value.
- **Logging** uses `slog` with the request's context (`slog.InfoContext(r.Context(), …)`), so the line carries the request ID.
- **Time.** Anything that expires, a window or a cache uses `Deps.Now`, so tests can move the clock.
- **Constants.** Limits, intervals and defaults get a named constant with a comment, next to the code that uses them.
- **Lint limits** (`.golangci.yml`): cognitive complexity 25; functions at most 80 lines or 50 statements. Tests are exempt. Split a long function by what each part decides, not into numbered steps.
- **Tables.** Each table belongs to one package, and a test enforces it ([Table ownership](#table-ownership)). A new table or column gets a new migration, never an edit to an old one, and the migration starts with a comment that says why.
- **SAM.** Each domain lives in one package, including how it is sent to SAM. The SAM package holds only the outbox, the client and token signing. A change SAM must reflect is written to the outbox in the same transaction, never sent to SAM from a handler.
- **Demo data** is a separate tool. It is never part of the service's packages or migrations.
- **Contract changes** start in the front end (`frontend/src/lib/services`). Then `openapi.yaml` is regenerated ([the API contract](#the-api-contract)).

## Table ownership

Each table is read and written only by the package that owns it ([Data ownership](design/README.md#data-ownership)); other packages call the owner's functions. A package other than the owner uses a table only where the last column names it, with the reason. `TestTableOwnership` reads this table and fails when code reads or writes a table it neither owns nor is listed against, when a migration creates a table missing here, when code outside migrations changes the schema without a reason in the test, or when a package other than `internal/db` takes an advisory lock itself.

| Table | Owner | Also used by |
|---|---|---|
| `schema_migrations` | `internal/db` | |
| `sam_outbox` | `internal/samaccess` | |
| `users` | `internal/accounts` | |
| `google_identities` | `internal/accounts` | |
| `sessions` | `internal/accounts` | |
| `demo_accounts` | `internal/accounts` | |
| `phone_challenges` | `internal/accounts` | |
| `phone_attempts` | `internal/accounts` | |
| `rate_limits` | `internal/accounts` | |
| `rate_events` | `internal/accounts` | |
| `sam_outbox_versions` | `internal/samaccess` | |
| `stages` | `internal/classrooms` | |
| `regions` | `internal/classrooms` | |
| `host_questions` | `internal/hosting` | |
| `platform_settings` | `internal/settings` | |
| `configuration_changes` | `internal/settings` | |

## Testing

```sh
make test         # every test, with the race detector
make test-short   # only those that need no database or SAM
make lint
```

Tests that need the database call `testutil.DB(t)`, which gives the test a database of its own, migrated and dropped when it ends, so tests run in parallel without seeing each other's rows. Each is a copy of a template migrated once per set of migrations. `DGS_TEST_DATABASE_URL` is an admin connection to the server they are created on, which must have PostGIS; `make test` starts the compose `test-db` and points it there unless the variable is already set. Under `-short` these tests are skipped.

### Integration tests

`apitest.New(t)` runs this service for one test, over HTTP, as the front end calls it:

1. **SAM.** `internal/testenv` builds `cmd/sam` from SAM's module at the version `go.mod` names, once per test run, and keeps the binary under the system's temporary directory by version. It gives SAM a new database on the test server, runs `sam bootstrap -json`, onboards this service with `sam tenant create -name dgs -registry`, naming this service's key set, and runs `sam serve` on a free loopback port until the test ends. A test that fails shows SAM's log.
2. **This service** gets a database of its own and listens on the address SAM was told about.
3. **The contract.** Every response to a request under `/api` is checked against `openapi.yaml`: its status must be one the operation declares, and its body must match the declared schema. Any operation may also answer `500` with the `internal` error. A breach, or a request for a path the contract lacks, fails the test. The key set (`/api/.well-known/`) and media files (`/api/media/`) lie outside the contract.

`e.SAM.Service()` and `e.SAM.Operator()` are SAM clients acting as this service and as the deployment's operator, for setting a test up or checking what reached SAM.

**Route coverage.** `TestRouteCoverage` lists every operation in the contract this service does not serve yet, and how many it does, from the service built as it runs, with its database and SAM, so every module is mounted. It reports and does not fail while the service is built.

**Contract gaps.** `apitest.Gaps` lists the statuses this service answers that an operation does not declare yet, each with why, such as `401` on `auth.getCurrentUser`. They belong in the front end's routes, which the contract is generated from; until they are there, the check lets them through, provided the body is an `Error`.

**Signing in** in a test: `e.Google.Token(person, e.Clock.Now())` is an ID token from the fake Google the service trusts, and `e.NewClient()` a browser with its own cookies. `e.Clock` is the service's clock, moved by hand. `apitest.NewWith(t, apitest.Options{Demo: true})` turns demo mode on. `e.SMS` keeps the texts the service sends, so a test reads a code from it, and `e.DoFrom` sends a request from a client address, as a trusted proxy would pass it.

### SAM's module

SAM's repository is private, and `go.mod` lists its command as a tool (`go get -tool`), which pins its version with the other modules. Go fetches it with git, so:

- `GOPRIVATE=github.com/dagrouper-school/*` keeps it away from the public module proxy and checksum database. The Makefile sets it.
- Locally, git reaches GitHub as the user the GitHub CLI is signed in as. `make up` passes the image build `SAM_READ_TOKEN`, from `gh auth token` unless it is set, as a build secret that never reaches an image layer.
- CI gets a token from a GitHub App installed on the SAM repository with read access to its contents and nothing else. Each job mints its own installation token (`actions/create-github-app-token`), limited to that repository, and it is revoked when the job ends. The app's client ID is the repository variable `SAM_APP_CLIENT_ID` and its private key the secret `SAM_APP_PRIVATE_KEY`. The token serves the tests, the vulnerability check and the image build.

To move to a newer SAM: `go get -tool github.com/dagrouper-school/sam/cmd/sam@<commit or tag>`, then `go mod tidy`.

### Rules

- **Unit tests** cover pure logic, in the package that holds it. Integration tests cover behaviour across the API, and are never the only test of a rule such as validation, permissions, cutoffs or alerts.
- **Every fix** comes with a test that fails without it.
- **Every endpoint** is called by an integration test in `internal/apitest`, so the contract check covers it.
- Tests call `t.Parallel()`.

CI (`.github/workflows/backend.yml`) runs on changes to `backend/`, the front end's services and domain, and the contract generator. It checks the contract is current, then runs `make tidy-check`, `make lint`, `make test` (against a PostGIS service container) and `make vulncheck`, and builds the image.

## The API contract

- [`openapi.yaml`](../openapi.yaml) is the OpenAPI 3.1 contract for every call the front end makes, and [`index.html`](index.html) shows it in Swagger UI, served from `swagger-ui/` with no CDN. Serve the folder over HTTP to view it, for example `python3 -m http.server -d backend` and open `/docs/`.
- Both are generated from the front end: `routes.ts` (method, path, access and body of each call), `services/types.ts` (descriptions and error codes) and `domain/types.ts` (the data types).
- After changing any of them, run `npm run openapi -w frontend` from the repository root. `npm run openapi:check -w frontend` fails when the committed files are out of date; CI runs it.
- Go types are written by hand and held to the contract by tests, not generated.
