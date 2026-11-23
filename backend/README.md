# Backend

The DaGrouper School backend: a Go service, and SAM's first tenant. The front end talks only to this service. It will hold accounts, sessions and everything specific to DaGrouper School, and call SAM for permissions and observations; SAM owns observations, their validation, quality flags and publication.

So far the service runs, serves `/healthz` and shuts down cleanly. The design it is built to is in [`docs/design/`](docs/design/README.md). Every page is listed in the [documentation index](docs/README.md).

## Running it

Needs Go 1.27, or Docker.

```sh
make run     # in development, on :8090
make up      # or with docker compose
curl -s localhost:8090/healthz
```

`make help` lists the other tasks. `make lint` and `make test` run the same checks as CI (`.github/workflows/backend.yml`). Configuration comes from the environment; every variable and its default is listed in [development](docs/development.md#configuration), with the make targets, the package layout and the conventions.

Every response carries an `X-Request-ID`, and every log line written while serving it names the same ID. Writes from an origin that is neither this service nor in `DGS_ALLOWED_ORIGINS` are refused.

## API contract

- [`openapi.yaml`](openapi.yaml): OpenAPI 3.1 contract for every call the front end makes, with request and response schemas.
- [`docs/index.html`](docs/index.html): the same contract in Swagger UI. Serve the folder over HTTP to view it, for example `python3 -m http.server -d backend` and open `/docs/`.

Both files are generated from the front end, so they always match what it sends:

- `frontend/src/lib/services/http/routes.ts`: method, path, access level and body of each call;
- `frontend/src/lib/services/types.ts`: the service interfaces, whose comments become the descriptions;
- `frontend/src/lib/domain/types.ts`: the data types, turned into JSON Schemas.

After changing any of them, run `npm run openapi` in `frontend/`. `npm run openapi:check` fails when the committed files are out of date, and CI runs it.

Errors use the front end's format, `{code, message, fields}`, with `code` one of its `ServiceErrorCode` values; an unexpected failure is `internal`, shown to the user as a generic message.
