# Development

How to build, run and test the service as it stands, and the conventions its code follows. So far the service has configuration, logging with request IDs, the HTTP conventions, `/healthz` and graceful shutdown.

## Prerequisites

| Tool | Version | For |
|---|---|---|
| Go | 1.25 (`go.mod`) | Building and testing |
| golangci-lint | v2 (CI pins v2.15.0) | `make lint` |
| Docker with Compose | Recent | `make up`, the image |
| Node and npm | 22 | Regenerating and checking the API contract, from the repository root |

## Running

```sh
make run                     # in development, on :8090
curl -s localhost:8090/healthz
```

Or with Docker:

```sh
make up                      # builds the image and starts it, waiting until healthy
make down                    # stops it
```

`compose.yaml` runs the service alone, in development, with `http://localhost:5173` as its front-end origin, published on `127.0.0.1:8090`.

## Make targets

The targets are the same as SAM's. `make help` lists them. The Makefile sets `DGS_ENV=development` unless the environment already sets it.

| Target | What it does |
|---|---|
| `help` | Lists the targets |
| `build` | Builds `bin/dgs` |
| `run` | Serves the API locally (`go run ./cmd/dgs serve`) |
| `test` | Runs every test with the race detector |
| `test-short` | Runs the tests that need no database or SAM (`-short`) |
| `lint` | `go vet`, a `gofmt` check and `golangci-lint` |
| `tidy-check` | Fails when `go.mod` or `go.sum` is not tidy |
| `vulncheck` | Reports known vulnerabilities in dependencies (`govulncheck`) |
| `up` | Builds and starts the compose stack, waiting until healthy |
| `down` | Stops the stack and discards its data |

## Configuration

Configuration comes from the environment (`internal/config`). Every problem is reported at once, and start-up stops on any.

| Variable | Default | Meaning |
|---|---|---|
| `DGS_ENV` | none: required (`make` sets `development`) | `development`, `test`, `staging` or `production`. Development conveniences exist only in the first two ([design](design/accounts-and-sign-in.md#environments)) |
| `DGS_LISTEN` | `:8090` | Listen address |
| `DGS_ALLOWED_ORIGINS` | `http://localhost:5173` in development and test; required elsewhere | Front-end origins allowed to call with credentials and to send writes, comma-separated. Each must be a bare origin such as `https://app.example`; a trailing `/` is dropped. Outside development and test each must use `https` |
| `DGS_LOG_FORMAT` | `text` in development and test, `json` elsewhere | `text` or `json` |
| `DGS_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |

Every variable the service reads is listed here.

## Package layout

| Package | Responsibility |
|---|---|
| `cmd/dgs` | The command. `dgs serve` (the default) serves until `SIGINT` or `SIGTERM`, then stops taking requests and waits up to ten seconds for those in flight |
| `internal/config` | Reads and checks the configuration |
| `internal/logx` | Sets up `slog` in text or JSON; carries the request ID so every log line written while serving a request names it |
| `internal/httpx` | The error format, `WriteJSON`, `WriteError`, the `Handler` type, strict decoding |
| `internal/server` | `server.New(Deps)`, the `Module` interface, the middleware, `/healthz` and the API's not-found answer |

The areas the code is organised into, and the data each owns, are in the design overview ([Data ownership](design/README.md#data-ownership)).

## The HTTP layer

Every request passes through, in order:

1. **Request ID.** A random ID, in the request's context and in the `X-Request-ID` response header.
2. **Request log.** One line per request with method, path, status and duration in milliseconds, except `/healthz`.
3. **CORS.** For an `Origin` in `DGS_ALLOWED_ORIGINS`: `Access-Control-Allow-Origin` with credentials and `Vary: Origin`. A preflight is answered `204` with methods `GET, POST, PUT, PATCH, DELETE`, header `Content-Type` and a ten-minute max age.
4. **Cross-site write check.** A request other than `GET`, `HEAD` or `OPTIONS` whose `Origin` is neither this service's own nor in `DGS_ALLOWED_ORIGINS` is refused with `403 forbidden`.
5. **Routes.** `GET /healthz` answers `{"status":"ok"}`. Modules mount their routes under `/api`. Any other path under `/api/` answers `404 not_found`.

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
- **Tables.** Each table belongs to one package, and a test enforces it: a query on another package's table fails unless the test lists it with its reason. A new table or column gets a new migration, never an edit to an old one, and the migration starts with a comment that says why.
- **SAM.** Each domain lives in one package, including how it is sent to SAM. The SAM package holds only the outbox, the client and token signing. A change SAM must reflect is written to the outbox in the same transaction, never sent to SAM from a handler.
- **Demo data** is a separate tool. It is never part of the service's packages or migrations.
- **Contract changes** start in the front end (`frontend/src/lib/services`). Then `openapi.yaml` is regenerated ([the API contract](#the-api-contract)).

## Testing

```sh
make test         # every test, with the race detector
make test-short   # only those that need no database or SAM
make lint
```

Every test so far is a unit test that needs nothing running. The integration harness, with a real SAM built from source and every response checked against `openapi.yaml`, is described in the [design overview](design/README.md#testing-approach).

- **Unit tests** cover pure logic, in the package that holds it. Integration tests cover behaviour across the API, and are never the only test of a rule such as validation, permissions, cutoffs or alerts.
- **Every fix** comes with a test that fails without it.
- **Every endpoint** is called by an integration test in `internal/apitest`, so the contract check covers it.
- Tests call `t.Parallel()`.

CI (`.github/workflows/backend.yml`) runs on changes to `backend/`, the front end's services and domain, and the contract generator. It checks the contract is current, then runs `make tidy-check`, `make lint`, `make test` and `make vulncheck`, and builds the image.

## The API contract

- [`openapi.yaml`](../openapi.yaml) is the OpenAPI 3.1 contract for every call the front end makes, and [`index.html`](index.html) shows it in Swagger UI, served from `swagger-ui/` with no CDN. Serve the folder over HTTP to view it, for example `python3 -m http.server -d backend` and open `/docs/`.
- Both are generated from the front end: `routes.ts` (method, path, access and body of each call), `services/types.ts` (descriptions and error codes) and `domain/types.ts` (the data types).
- After changing any of them, run `npm run openapi -w frontend` from the repository root. `npm run openapi:check -w frontend` fails when the committed files are out of date; CI runs it.
- Go types are written by hand and held to the contract by tests, not generated.
