# SAM integration

How this service, as SAM's first tenant, keeps SAM in step with its own database, whose authority each call carries, and how SAM's answers reach users.

## Context

This service's database is the record for people, classrooms, memberships, labs, devices and sharing settings. SAM holds what is derived from them: principals, attributes, spaces, groups, memberships, grants, devices, data policies and publications. The two are separate databases, so no transaction spans both.

If attributes were sent to SAM in two ways, synchronously at sign-in and by a background retry loop, `consent` could arrive out of order: SAM would keep `given` after a withdrawal, while this database recorded `withdrawn` as sent. Nobody would notice without something comparing the two, and sign-in would wait on SAM. Classrooms make it harder: creating one means a space, four groups and their grants.

SAM also checks the actor of every change. It refuses a grant or membership to oneself and a principal asserting attributes about itself, and its audit names the actor. If every change reached SAM as this service, those checks would protect nothing and the audit would name the service.

## Goals

- Every change SAM must reflect is delivered in order per entity, and applied once.
- No request waits on SAM to write.
- SAM converges after an outage, a lost answer or a change made in SAM by hand, where it can be compared.
- What SAM refuses a person, it refuses them through this service too, and its audit names them.
- A restored database never gives one person's SAM identity to another.
- SAM's refusals reach users as the right error, never as a failure of this service.

## Non-goals

- A transaction across both databases.
- SAM reading this database, or this service reading SAM's.
- Reconciling what SAM cannot list (a group's members).
- Sending readings through the outbox: a submission's answer is the user's answer ([Labs and readings](labs-and-readings.md#relaying-readings)).

## Design

### The tenant

The service is onboarded once with `sam tenant create -json`, whose output is the tenant file: the service key, the tenant root space, the trusted issuer and SAM's anonymous principal. The service principal holds the tenant grant on the root space, from which it issues every role group's grants, and `registry:manage` there.

### Three ways to call SAM

| As | How | Used for |
|---|---|---|
| This service | Service key | Changes this service makes by policy; checks before a change |
| A person | Service key, a user token for them, and `SAM-Act-As` | Reads as the viewer; changes a person makes; relayed submissions |
| Anonymous | SAM's anonymous principal | Public pages |

SAM decides and audits a call made as a person as that person, with this service as `on_behalf_of`.

### User tokens and the key set

- For each call made as a person, this service signs a short-lived token: Ed25519, five minutes (SAM accepts at most ten), with the participant ID as subject, the issuer and audience the tenant's onboarding named, and this service's name in SAM as `azp`.
- The public keys are published at `/api/.well-known/jwks.json`. SAM fetches and caches the set.
- **Rotation:** SAM keeps a fetched set for five minutes, so a new key is published, signing nothing, at least that long before it signs; then it becomes the signing key and the old one stays published. Tokens last minutes, so the old key can be dropped from the set a day later. Rotation never strands a token, since SAM's cached set holds both keys ([Operations](../operations.md#rotating)).
- **Keys outside development:** start-up is refused without a configured signing key. A key generated at start-up exists only in development and test.

### Who acts in SAM

| Change | Sent as | Why |
|---|---|---|
| Registering a user's principal | This service | Policy: every account has one |
| Asserting `minor` and `consent` after the guardian's link | This service | Policy: they follow the consent flow, not the user's own word, and SAM refuses self-assertion |
| Asserting `consent` from a paper sheet | The person who checked the sheet, scoped to the classroom's space | They vouch for it; the policy assertion follows |
| A classroom's space, groups and grants; a lab's space and site | This service | Structure follows this service's rules |
| A membership change | The person whose decision it carries out, or this service for a platform admin with no role in the classroom | [Classrooms and permissions](classrooms-and-permissions.md#membership) |
| Registering or removing a device | The person who did it | SAM checks `device:register` or `device:maintain` |
| A classroom's data policy | The head who set the terms, or the platform admin who created a classroom with no head yet | SAM lets a tenant delegate `dataset:set_policy` but not use it |
| A release | The person releasing | SAM checks `dataset:publish` and records the publisher |
| Public grants | This service | They follow the sharing settings, not a person's rights |
| This tenant's registry entries | This service | It holds `registry:manage`; the admin is recorded in this service's change log |

Rules:

1. **The actor must still exist.** A row whose actor has been deleted is held for an operator. It is never sent as the service instead.
2. **No fallback.** When SAM refuses a person, the change is held and the person is told, as for any refused action. This service's authority never stands in for theirs.
3. **Collusion between platform admins is out of SAM's reach.** Two admins can add each other to a group that reads data, as can one admin with two accounts. SAM records each step against the person who took it and this service records who did what. Such changes are caught by reviewing the audit, not prevented.

### The outbox

Every change SAM must reflect writes an outbox row in the same transaction as the change.

| Column | Holds |
|---|---|
| `entity` | What the row is about, such as `user:<id>` or `membership:<classroom>:<user>` |
| `kind` | Which applier sends it |
| `version` | Increases per entity |
| `payload` | The entity's whole desired state, not a delta |
| `actor_id` | The person it is sent as, or none for this service. A kind declares whether its rows carry one |
| state, attempts, next attempt, last error | The worker's bookkeeping |

The worker:

- **Runs on one replica,** chosen by an advisory lock, and checks for work every second.
- **Works each entity on its own,** so one held or backed-off entity never delays another.
- **Sends in version order per entity.** Only the latest pending version is sent: since each payload is the whole desired state, the rows it supersedes, waiting or held, are marked superseded unsent once SAM holds it.
- **Retries with backoff,** up to five minutes between attempts.
- **Waits for dependencies.** An applier returns "not yet" when another entity must reach SAM first, such as a classroom's groups before a membership, or a head's own membership before rows sent as that head. The row is retried, not held.
- **Holds what retrying cannot fix:** a refusal from SAM, a missing actor, a payload or kind this version cannot read. Held rows stay until an operator releases them.

Operator commands:

```sh
dgs outbox held                # what is held, with the error
dgs outbox release ENTITY      # send its latest version again on the next pass
```

The kinds:

| Area | Kinds |
|---|---|
| Accounts | user (principal, `minor`, `consent`), admins group, admin membership |
| Registry | this tenant's registry domains, parameters and device types |
| Classrooms | classroom (space, groups, grants), membership, attribute assertion |
| Labs | lab (space and site), device |
| Sharing and release | sharing (public grants), data policy, release |

**No request waits on SAM.** Sign-in creates the account and queues it; the worker registers it, normally within a second. Until then the person sees what a person without grants sees.

**SAM lags behind.** SAM reflects a change only once it is delivered, and until then keeps the state before it, which may be more permissive (consent still `given` after a withdrawal). Whatever this service relays to SAM for a person therefore checks its own record first.

### Idempotency keys

Every request the worker sends carries an idempotency key that names the change: the entity, what is changed and the version, such as `user:<id>:attr:consent:v3`.

- Keys never use the outbox row ID or the participant ID, which a restored database could give out again.
- Keys carry no attempt number. Releases, data policies, public grants and device registrations use stable keys: a retry after a lost answer gets the first answer back, so a release is never published twice.
- SAM does not replay a refusal for the same key, so a retry after a refusal is checked afresh, for example once the person's membership has arrived.

### The reconciler

Every fifteen minutes, on one replica, the reconciler compares what SAM holds with this database and enqueues a correction when they differ.

| Entity | Compared | How it converges |
|---|---|---|
| Users | Principal and attributes | Re-queues the user's desired state |
| Sharing | The live anonymous grants on each classroom's space | Each public grant carries the label `classroom-public` and one naming it (`public:<name>`). The applier revokes grants the settings do not call for and duplicates left by a lost answer, and issues the missing ones |
| Memberships | Not compared: SAM cannot list a group's members | The applier records each step SAM confirms: removal from the old group, then addition to the new one. A retry resumes from the last confirmed step, and a member is never in two role groups at once |

A user it cannot compare is logged and tried again on the next pass. It reads users a page at a time.

### Participant IDs and the restore guard

- The participant ID is the subject of a person's SAM principal and the only identifier of a person SAM holds. It comes from a sequence and is never reused.
- A principal is registered only for an account with none stored, under an idempotency key naming the account and the change's version.
- **Adoption.** When an answer is lost, the retry carries the same key and SAM replays its first answer: the account's own principal, which is adopted. SAM answers `409 already_exists` only when the participant ID is a principal registered by some other request: one belonging to an account this database does not know, which happens only after a restore. That is never adopted: the row is held for an operator, who gives the account a new participant ID. SAM replays an answer for 24 hours, so a lost registration retried after that is held the same way.
- **At start-up** the guard moves the sequence past every ID SAM already knows. IDs are given in order, so the registered ones above the next ID are a run: the guard finds its end by doubling, then halving, with a bounded number of lookups, and logs a warning when it moves the sequence. While SAM cannot be asked, the service starts anyway, the guard retries in the background, and registrations wait for it.

### Reads

SAM offers four authorised reads: list, latest, aggregate and summary. This service calls them as the viewer, or as anonymous for a visitor, and builds the front end's view models from the answer ([Dashboards and public site](dashboards-and-public-site.md)). SAM's own read rules (licences, location, identity) therefore apply to every page.

### Translating SAM's errors

| SAM answers | The front end gets | Note |
|---|---|---|
| 400 or 422 (`invalid_request` and an endpoint's own) | `invalid_input` | With field messages where SAM names the field and the front end has one |
| 401 or 403 | `forbidden` | Never `unauthenticated`: the session is this service's. A 401 is a fault in this service's credentials, logged as such |
| 404 | `not_found` | |
| 409 | `conflict` | |
| 429 | `rate_limited` | With `Retry-After` |
| 5xx, timeout, unreachable, or a call SAM refuses as malformed | `internal` | Logged with the request ID |

The message is this service's own. SAM's developer-facing detail is logged, never shown. A SAM refusal is never answered as a failure of this service. Calls to SAM have a timeout of fifteen seconds.

## Alternatives considered

- **Synchronous writes from the request.** Simple, but sign-in would wait on SAM, an outage would fail every change, and two paths (in the request and in a retry loop) could deliver `consent` out of order. Rejected for the outbox.
- **Sending every change as this service.** SAM's checks on the actor would protect nothing, and its audit would name the service. Rejected: a person's change is sent as that person.
- **Falling back to this service's authority when a person's does not suffice.** It would let a person do through this service what SAM refuses them, such as a head making heads. Rejected.
- **Idempotency keys from row IDs or participant IDs.** A restored database gives them out again, and SAM would replay an answer meant for another change. Rejected for keys that name the change.
- **A key per attempt.** A retry after a lost answer would publish a release twice. Rejected for stable keys.
- **Adopting a principal that already exists.** After a restore, one person would act as another. Rejected: only the replayed answer to this account's own request is adopted.
- **Reconciling memberships.** SAM cannot list a group's members, so there is nothing to compare. The applier records confirmed steps instead.

## Risks

| Risk | Mitigation |
|---|---|
| SAM keeps a more permissive state until delivery | Relays check this service's own record first; delivery normally takes a second |
| Held rows go unnoticed | `dgs outbox held`; held rows are logged at warning level |
| A membership changed in SAM by hand is not repaired | Only this service changes classroom groups; SAM's audit shows any other change |
| The signing key is lost or leaked | Rotation without downtime; tokens live five minutes |
| The outbox grows without bound | Done rows are pruned; the period is an open question |
| Reconciling every user grows with the user count | Paged reads; the interval is configurable |

## Testing approach

- **Unit:** idempotency key naming, supersession, ordering per entity, backoff, the hold rules, the restore guard's search, error translation.
- **Integration** against a real SAM: SAM stopped and started again converges; a lost answer is not applied twice; a deleted actor's row is held; a reused participant ID is held and the sequence moves past it; a refusal reaches the user as `forbidden`, never `internal`.
- **Token rotation:** tokens signed by a retired key still verify while it is published.

## Open questions

- How long done outbox rows are kept, and which metrics an operator alerts on.
- Whether the reconciler should also compare classrooms' spaces, groups and grants, which SAM can list.
- How sensors reach SAM: directly, or relayed. The signed reading envelope works by either path.
