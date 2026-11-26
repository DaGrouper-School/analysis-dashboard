# Accounts and sign-in

How people get an account and a session, how their profile is completed, and which conveniences exist only in development. The front end's `AuthService`, `ProfileService` and `PlacesService` are the contract.

## Context

Participants sign in with Google or with a code texted to their phone. Many participants in the pilot countries have a phone but no Google account. Google sign-ups verify a phone number during onboarding, and everyone completes a profile and address before reaching the dashboard.

A six-digit code with five attempts is safe only if the attempts really are limited. A check that reads the attempt count, compares the code and then increments it, as separate statements, lets parallel guesses all see zero attempts. If a new code reset the count, the limit would mean nothing. Without a limit on how often codes are sent, guessing works at scale, and so does SMS pumping, where an attacker triggers paid messages to numbers they profit from.

If the environment defaulted to development, a deployment that forgot to set it would sign SAM tokens with a key generated at start-up and write one-time codes to its logs. Demo mode, which returns any number's code to the caller and offers sign-in as the demo platform admin, would open every account.

## Goals

- No passwords. Google proves an email; a code proves a phone.
- Sign-in never waits on SAM ([SAM integration](sam-integration.md#the-outbox)).
- Codes resist guessing and SMS pumping, across replicas.
- A database leak gives no sessions and no link tokens.
- Every development convenience is refused outside development and test.

## Non-goals

- Email and password sign-in, or multi-factor sign-in.
- Merging a Google account and a phone account that belong to the same person.
- Keeping Google's tokens: the ID token is checked once and dropped.

## Design

### Accounts

- An account has a participant ID ([SAM integration](sam-integration.md#participant-ids-and-the-restore-guard)), a name, an email, a phone, an address, the admin flag, the minor and consent state, and the public profile choice.
- Every path that creates an account writes the user's outbox row in the same transaction, so no account exists that SAM will not hear of.
- An email is unique only when verified, that is, when it came from Google. A Google sign-in whose email belongs to another account is refused with `conflict` and asks the person to sign in the way they did before.
- A Google identity is linked to its account by Google's subject (`google_identities`), so a change of email at Google keeps the account.
- Until onboarding says, an account is shown as no minor with consent not needed, as the front end starts every account. SAM treats the person as a minor until onboarding asserts `minor=false`, so nothing they submit is accepted before then.

### Google sign-in

The front end gets an ID token from Google Identity Services and posts it. This service verifies:

- the signature, against Google's published keys, cached for an hour and refetched for an unknown key at most once a minute;
- the issuer, `accounts.google.com` or `https://accounts.google.com`, both of which Google issues;
- the audience, `DGS_GOOGLE_CLIENT_ID`;
- the expiry, with thirty seconds' leeway.

The first sign-in creates the account and returns `isNewUser`, which sends the person to onboarding.

### Phone codes

| Rule | Value |
|---|---|
| Code | Six digits, from a secure random source, stored hashed |
| Lifetime | Five minutes |
| Resend | Not before thirty seconds |
| Attempts | Five per number for the code's lifetime window; a new code does not reset them |
| Purposes | `sign_in`, and `verify_phone` for a signed-in Google user during onboarding |

**Atomic checks.** A verification increments the attempt counter and reads the result in one statement, `UPDATE … SET attempts = attempts + 1 WHERE … AND attempts < max RETURNING …`. The code is compared only after that increment succeeds. A failed attempt is counted before the code is checked, so a wrong guess is never free.

**Send and verify limits,** over sliding windows counted in the database so they hold across replicas:

| Key | Sends | Verifications |
|---|---|---|
| Phone number | 5 per hour | 10 per hour |
| Client address | 20 per hour | 50 per hour |
| Overall | 200 per hour (`DGS_SMS_SENDS_PER_HOUR`) | none |

- A refusal is `429 rate_limited` with `Retry-After`.
- The client address is the connection's, or the last value of `DGS_CLIENT_IP_HEADER` when a trusted proxy sets it. IPv6 addresses are keyed by their /64.
- Numbers are normalised to E.164 before any limit or lookup.

With these limits, guessing one code needs on the order of 10^5 numbers' worth of budget, and the overall limit caps the cost of SMS pumping. It is set per deployment from the expected sign-up rate.

### Sessions and cookies

- A session is an opaque random token. The cookie holds the token; the database holds its hash.
- The cookie is `session`: `HttpOnly`, `SameSite=Lax`, and `Secure` whenever the service is reached over HTTPS (`DGS_PUBLIC_URL`).
- Sessions slide. An unused session ends after `DGS_SESSION_TTL` (30 days by default). Using it extends it, at most once an hour, and the cookie is sent again with the new expiry so the browser keeps it as long as the database does.
- Signing out deletes the session and clears the cookie.
- `RequireUser` wraps every route that needs a signed-in user; a request without a valid session gets `401 unauthenticated`. `GET /auth/me` answers 401 for no session, which the front end reads as signed out.
- **Cross-site requests.** Writes from an origin that is neither this service nor in `DGS_ALLOWED_ORIGINS` are refused, and request bodies must be JSON. Browsers send `Origin` on every cross-origin write and cannot send JSON across origins without a preflight this service refuses, so the front end needs no CSRF token.

### Profile and onboarding

- `updateProfile`, `updateAddress` and phone verification fill the profile. `completeOnboarding` checks the required fields and sets `onboardingCompleted`.
- **Places.** Address autocomplete and lookup go through a provider behind an interface, so the provider's key stays on the server. The provider is Google Places, whose suggestions the front end's main and secondary text are shaped after. Development and test use an offline provider with a fixed list covering the pilot regions, the same as the front end's mock. `dialCodes` lists calling codes, pilot countries first.
- **Public profile.** `setPublicProfile` shows the person's name and role next to their participant ID on contributor pages. It is refused for a minor ([Dashboards and public site](dashboards-and-public-site.md#contributors-and-credit)).
- **Minors.** `markMinor` and `requestGuardianConsent` mark the account as a minor's and start consent ([Classrooms and permissions](classrooms-and-permissions.md#guardian-consent)).

### Platform admins

A platform admin runs the programme: they set up classrooms and labs, decide host applications, keep the registry and the platform's configuration, register devices and manage members, and read nothing as admins ([Classrooms and permissions](classrooms-and-permissions.md#classrooms-in-sam)).

- **Who decides.** `users.platform_admin` is the record, and only an operator changes it, from a terminal: `dgs admins add` and `dgs admins remove`, naming the person by their Google email or account ID once they have signed in. Each change is logged in `configuration_changes`. Admins do not make admins from the dashboard, so one compromised account cannot make more.
- **In SAM** they are one group at the tenant root, holding the admins' grants there. This service creates the group, issues its grants and keeps its members in step with the record, as itself: an operator, not the admin, decided.
- **At once here.** Every check in this service reads the record, so an admin who is no longer one loses access at once. SAM follows through the outbox, as a change that takes access away.
- **The reconciler** asks SAM, as each admin and each former admin, whether they may register devices at the tenant root, and corrects a membership SAM holds otherwise. Heads hold that right only on their classroom's space.

### Delivery

Codes, invitations, guardian links and host-application links are sent through `notify`: an SMS provider and an email provider behind interfaces, so a module never names a provider.

- **Email** goes through an SMTP server, so any mail service that offers SMTP can carry it. Outside development and test a server is required and the connection is encrypted.
- **SMS** has no provider: outside development and test, sending a code is refused and the error is logged.
- **Development and test** use providers that write each message to the log, which no other environment may.

### Environments

`DGS_ENV` is required and has no default. It is one of `development`, `test`, `staging` or `production`.

| Behaviour | development, test | staging, production |
|---|---|---|
| Missing `DGS_ENV` | Start-up refused | Start-up refused |
| `DGS_ALLOWED_ORIGINS` | Defaults to `http://localhost:5173`; `http` allowed | Required; `https` only |
| Signing key | Generated when missing | Required, or start-up is refused |
| Demo mode (`DGS_DEMO`) | Allowed | Start-up refused |
| One-time codes in the log | Yes | Never |
| Email | To the log, or an SMTP server, which may be unencrypted | An SMTP server, encrypted; required |
| Log format | `text` | `json` |

**Demo mode** offers three conveniences, each only with `DGS_DEMO` on:

- the demo Google accounts, from `demo_accounts`, signed in by their subject instead of a Google token (`demoGoogleAccounts` returns none otherwise);
- the phone code returned in the response instead of texted;
- link tokens (invitation, guardian consent, host application) returned to the client in their `token` field, which is otherwise empty.

Staging gets demo-like data from seeded accounts that sign in normally.

### Demo data

The demo people, classrooms, labs, devices and their history come from the front end's seed. They are loaded by a separate command, `dgs-demo`. Its code lives under `cmd/dgs-demo`, and the service's packages never import it.

- `dgs seed` stays in the service and loads only configuration: the registry overlay, stages, regions, host questions, settings and the first consent text. It is safe in every environment.
- `dgs-demo` loads the demo people and classrooms, sends their history to SAM as the people who recorded it, and releases it. A re-run keeps later releases and changes.
- It refuses to run outside development and test, and needs the backend running, since SAM checks the senders' tokens against this service's key set. It reads the signing key from a read-only mount and never creates one.
- The compose stack runs it once the backend is healthy, so one `make up` gives a seeded stack.
- Its two tables, `demo_accounts` and `seed_history`, stay empty in production. Demo data gets no migration of its own.

## Alternatives considered

- **`DGS_ENV` defaulting to development.** Convenient locally, unsafe when forgotten. Rejected: a missing variable stops start-up with a clear message.
- **Reading, comparing and incrementing attempts in separate statements.** Parallel guesses all pass. Rejected for one atomic statement.
- **An attempt budget per code.** Requesting a new code would reset it. Rejected for a budget per number.
- **Limits in memory.** Each replica would count alone. Rejected for counts in the database.
- **Signed, stateless session cookies.** They cannot be revoked one by one, and a stolen one lasts until it expires. Rejected for opaque tokens stored hashed.
- **Demo data loaded by the service.** Seed flags and a history sender inside the service would put made-up data in its packages, guarded only by environment checks. Rejected for a separate tool.

## Risks

| Risk | Mitigation |
|---|---|
| SMS cost from abuse | The overall send limit, set per deployment |
| A recycled phone number reaches another person's account | A code proves only the phone; see the open questions |
| Shared tablets in classrooms keep a session for 30 days | Sign out is one tap; the session lifetime is configurable |
| Google's keys cannot be fetched | Cached for an hour; sign-in by phone still works |
| A deployment runs with development conveniences | Refused at start-up |

## Testing approach

- **Unit:** number normalisation, the sliding windows, code expiry and resend, configuration refusals per environment, the Google token checks with a fake key set.
- **Integration:** parallel wrong guesses spend the budget exactly; a new code keeps the attempt count; the sixth send to one number within an hour answers `429` with `Retry-After`; sign-in succeeds while SAM is stopped and the account reaches SAM once it is back; sessions slide and end; a demo account is refused without demo mode.

## Open questions

- Whether a Google account and a phone account of the same person can be joined, and how.
- Whether shared classroom tablets need a shorter session.
- Which SMS and email providers serve the pilot countries.
- Whether a minor may sign in with Google, given Google's own age rules.
- What protects an account whose phone number is given to someone else by the carrier.
