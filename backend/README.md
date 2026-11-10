# Backend

Placeholder for the DaGrouper School backend. The stack is not chosen yet.

It will provide the auth API (Google and phone sign-up, sessions in an HTTP-only cookie) and the
link to SAM, which owns validation, calculations, quality flags, permissions and publication. The
front end never calls SAM directly.

## API contract

- [`openapi.yaml`](openapi.yaml): OpenAPI 3.1 contract for every call the front end makes, with
  request and response schemas.
- [`docs/index.html`](docs/index.html): the same contract in Swagger UI. Serve the folder over HTTP
  to view it, for example `python3 -m http.server -d backend` and open `/docs/`.

Both files are generated from the front end, so they always match what it sends:

- `frontend/src/lib/services/http/routes.ts`: method, path, access level and body of each call;
- `frontend/src/lib/services/types.ts`: the service interfaces, whose comments become the
  descriptions;
- `frontend/src/lib/domain/types.ts`: the data types, turned into JSON Schemas.

After changing any of them, run `npm run openapi` in `frontend/`. `npm run openapi:check` fails
when the committed files are out of date, and `routes.spec.ts` fails when the HTTP client and
`routes.ts` disagree.

To run the front end against a backend instead of the mock, set `VITE_DATA_SOURCE=http` and
`VITE_API_URL` (default `/api`).
