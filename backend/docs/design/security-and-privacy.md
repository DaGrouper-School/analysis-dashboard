# Security and privacy

The risks this service guards against, the rule for each, and the data-protection points. Each rule comes with a test that fails without it.

## Context

The service holds personal data about participants, many of them minors, and their guardians. It signs tokens SAM trusts, so a flaw here can become a flaw in SAM's decisions. It sends paid SMS on request from the open internet. And it publishes data about places where children work.

## Goals

- No data permission is decided here alone: SAM decides, as the person.
- No secret is stored in plain text.
- No development convenience runs outside development and test.
- Minors are protected by several independent rules, not one.
- Personal data is minimised, and SAM holds none beyond the participant ID.

## Non-goals

- Protecting against a compromised operator with database and key access.
- Preventing collusion between platform admins: it is recorded and caught by audit.

## Design

### Threat model

| Threat | Rule |
|---|---|
| Cross-site request forgery | Session cookie `SameSite=Lax`; writes from origins other than this service or `DGS_ALLOWED_ORIGINS` refused; JSON bodies only |
| Credentials sent to the wrong site | CORS with credentials only for configured origins, which must use `https` outside development and test |
| A deployment with unsafe defaults | `DGS_ENV` required; demo mode, generated keys and logged codes refused outside development and test |
| Oversized or malformed requests | Bodies at most 1 MiB unless an endpoint allows more; unknown fields and trailing data refused; server read and idle timeouts |
| Internal detail in error responses | Unexpected errors answered as `internal` with a generic message, logged with the request ID; SAM's detail never shown |
| Guessing a phone code | Attempts incremented atomically before the code is compared; one budget per number, not per code |
| SMS pumping | Send limits per number, per client address and overall |
| One client posing as many | Client address from the connection, or from a trusted proxy's header only when configured; IPv6 keyed by /64 |
| Sessions stolen from a database leak | Session tokens stored as hashes |
| Link tokens stolen from a database leak or a response | Tokens stored as hashes; sent only to the recipient; returned to the client in demo mode only |
| A forged Google identity | Signature, issuer, audience and expiry checked against Google's keys |
| This service doing in SAM what a person may not | Changes and reads sent as the person; SAM's checks apply; no fallback to this service's authority; the grid checked against SAM cell by cell |
| A change sent for someone who has been deleted | Held for an operator, never sent as the service |
| Platform admins reading data | The admins group holds no read grant and never `dataset:publish` |
| A head making heads | Membership changes limited to what the changer's role hands on; making a head is an admin's task |
| An invitation outliving its authority | Invitations expire, are revoked with the inviter's role, and are accepted as the inviter |
| One person acting as another after a restore | Participant IDs never reused; principals tagged with their account; a foreign principal never adopted; the sequence moved past SAM's IDs at start-up |
| Signing key compromise | Ed25519 keys from configured files; five-minute tokens; rotation without downtime |
| A withdrawal, removal or end of public sharing not yet in SAM | Every call for a person or a visitor checks this service's own record first, and a held change that takes access away is logged at error level |
| Identifying a minor | Never named or listed; no public profile; GPS rounded before sending; positions generalised and identities stripped for visitors |
| Enumerating contributors | Contributor pages only for non-minors with a public profile; every other ID answers 404 like an unknown one |
| Publishing an email | Contributor profiles carry `email: null` |
| Unreleased data reaching visitors | Visitors read as SAM's anonymous principal, whose grants cover published records only |
| A provider key in the browser | Address lookup proxied through this service |
| Uploaded files used as an attack | Size-bounded, checked to be images, stored under content-hash names, served with a fixed type |
| Secrets in logs | Codes logged only in development and test; tokens and cookies never logged |

### Data protection

**What is held where.**

| Data | Held by | Who sees it |
|---|---|---|
| Name, email, phone, address | This service | The person; a classroom's head, facilitators and platform admins see members' emails |
| Participant ID | This service and SAM | Members; visitors on credited records |
| Minor and consent state | This service; SAM as attributes | The person; the classroom's head and facilitators |
| Guardian's name and contact, paper-sheet records | This service | The classroom's head and facilitators |
| Sessions, code attempts, link tokens | This service, hashed | Nobody |
| Readings and observations, with positions | SAM | As SAM's grants allow; visitors see published records, generalised and stripped |
| Who recorded each relayed record, who asked for credit | This service | Public pages show credited non-minors with a public profile only |
| Institution photos | This service's media store | Everyone, once set |

**Points.**

- **Minimisation.** SAM holds no name, email or phone: only the participant ID and the attributes `minor` and `consent`.
- **Consent.** A minor's data is collected only with a guardian's consent, recorded with how and by whom it was given and the version of the text agreed to. The text is published by platform admins as a logged version, never changed once published, and a minor's positions are never rounded more finely than it states. Withdrawal stops new submissions at once here and in SAM once delivered.
- **Processors.** Google (sign-in), an SMS provider, an email provider, an address provider and object storage. Each receives only what its task needs.
- **Logs** carry request IDs, methods, paths, statuses and durations; no request bodies.
- **Production gate.** Real data from minors does not enter production until SAM's consent withdrawal, deletion and retention are in place, together with the data-protection review.

## Alternatives considered

- **Enforcing data permissions here and calling SAM as the service.** One bug here would expose every classroom, and SAM's audit would name the service. Rejected: SAM decides as the person.
- **A CSRF token.** Needs state in the front end and every form. Rejected: `SameSite=Lax`, the origin check and JSON-only bodies stop cross-site writes without one.
- **Generated keys as a fallback outside development.** A forgotten configuration would sign tokens with a key nobody holds after a restart. Rejected: start-up is refused.

## Risks

| Risk | Mitigation |
|---|---|
| A rule exists but is not tested | Every rule here ships with a test that fails without it |
| Data from minors reaches production too early | The production gate above |
| Platform admins collude | Recorded by SAM and this service; reviewed |
| Personal data kept longer than needed | Retention periods are an open question, settled before production |

## Testing approach

- Each row of the threat model has a test, unit where the rule is pure (limits, configuration refusals, origin checks), integration where it spans SAM (acting as the person, anonymous reads).
- The origin and CORS rules are tested in `internal/server`, configuration refusals in `internal/config`.
- `make vulncheck` reports known vulnerabilities in dependencies, in CI.

## Open questions

- Retention periods for accounts, sessions, code events, consent records and relayed-record links, and how a person's data is deleted here and in SAM.
- How a person asks for a copy of their data.
- Who carries out the data-protection review, and when.
- The age of consent in each pilot country.
