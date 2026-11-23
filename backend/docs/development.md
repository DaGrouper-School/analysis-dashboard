# Development

How to build, run and test the service as it stands, and the conventions its code follows. So far the service has configuration, logging with request IDs, the HTTP conventions, its PostgreSQL database with migrations, calls to SAM as itself, as a person or as anonymous, the key set SAM verifies user tokens with, `/healthz`, graceful shutdown, and integration tests against SAM and the API contract.

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
| `DGS_SIGNING_KEYS_FILE` | none in development and test, where a key is generated at start-up; required elsewhere | The key file user tokens are signed with ([Operations](operations.md#signing-keys)) |

Every variable the service reads is listed here. The tests also read `DGS_TEST_DATABASE_URL` ([Testing](#testing)).

## Package layout

| Package | Responsibility |
|---|---|
| `cmd/dgs` | The command. `dgs serve` (the default) applies new migrations, reads the tenant file and signing keys, then serves until `SIGINT` or `SIGTERM`, then stops taking requests and waits up to ten seconds for those in flight. `dgs migrate` applies new migrations and exits. `dgs keys new` prints a new signing key ([Operations](operations.md#signing-keys)) |
| `internal/config` | Reads and checks the configuration |
| `internal/logx` | Sets up `slog` in text or JSON; carries the request ID so every log line written while serving a request names it |
| `internal/httpx` | The error format, `WriteJSON`, `WriteError`, the `Handler` type, strict decoding |
| `internal/db` | Opens the pool, applies the migrations, transactions (`InTx`) and advisory locks |
| `internal/samaccess` | Calls SAM as this service, as a person through a user token or as anonymous; the signing keys and the key set; SAM's errors translated |
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

## The database

PostgreSQL 17, through `pgx` (`internal/db`). `db.Open` opens a pool of at most `DGS_DB_MAX_CONNS` connections, each with `statement_timeout` set to `DGS_DB_STATEMENT_TIMEOUT`, and fails unless the database answers.

**Migrations** are SQL files in `internal/db/migrations`, embedded in the binary and applied in name order, each in its own transaction and recorded in `schema_migrations`. Every instance applies them on start, before it listens; an advisory lock makes the others wait while one applies them, and a failed migration stops start-up with nothing of it applied. The rules for writing one are in [the migrations folder](../internal/db/migrations/README.md).

**Transactions.** `db.InTx(ctx, q, fn)` runs `fn` in a transaction. When `ctx` already carries one (`db.WithTx`), `fn` runs in a savepoint of it, so a service function that needs a transaction works the same alone or inside a caller's.

**Advisory locks** are taken only through `db.Lock` (held by a connection until released) and `db.TryLock` (held until the transaction ends), and every key is a constant in `internal/db/locks.go`, so no two uses share one by accident.

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

**Route coverage.** `TestRouteCoverage` lists every operation in the contract this service does not serve yet, and how many it does. It reports and does not fail while the service is built.

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
