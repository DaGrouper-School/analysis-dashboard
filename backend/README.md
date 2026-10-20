# Backend

Placeholder for the Analysis Platform backend. The stack is not chosen yet.

It will provide the auth API (Google and phone sign-up, sessions in an HTTP-only cookie) and the
link to SAM, which owns validation, calculations, quality flags, permissions and publication.

The front end already talks to the backend through the service interfaces in
`frontend/src/lib/services/types.ts`. `frontend/src/lib/services/http/index.ts` lists the endpoints
it expects (`/auth/*`, `/me/*`, `/places/*`, `/sam/*`); treat those as a proposal to agree on.
