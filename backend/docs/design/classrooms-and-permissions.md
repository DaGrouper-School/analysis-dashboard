# Classrooms and permissions

How classrooms, their roles and their members are kept here and enforced by SAM, and how invitations, guardian consent and host applications work. The front end's `ClassroomService`, `ConsentService`, `HostingService` and `frontend/src/lib/domain/permissions.ts` are the contract.

## Context

Classrooms are the first DaGrouper concept SAM must enforce: who may read, read unreleased data, record, register devices, release and record consent. Some of the people who change a classroom hold no rights in SAM: platform admins manage but do not read. Others hold rights in SAM only once their own membership has arrived there.

The front end decides every permission with one function, `permissionsFor(viewer, role, sharing)`. Its result is sent to the UI as each classroom's permission flags, so the UI and the backend agree on one grid.

## Goals

- SAM refuses a person what the grid refuses them, through this service too.
- Joining, changing role and leaving change only that member's group membership. Nobody's departure voids anyone else's access.
- A change of sharing settings touches one group grant, not every member.
- An invitation never outlives the authority that issued it.
- A minor's consent is recorded with who gave it and the text they agreed to, and SAM enforces it.

## Non-goals

- Per-person grants.
- Classroom roles other than the four in the grid.

## Design

### The permission grid

`permissionsFor` in Go (`internal/roles`) is the same grid as the front end's. A non-member platform admin is "Admin"; a visitor has no viewer and gets nothing here, only the public pages.

| Permission | Admin | Head | Facilitator | Community scientist | Researcher | Enforced by |
|---|---|---|---|---|---|---|
| `view` | ✓ | ✓ | ✓ | ✓ | ✓ | This service |
| `seeUnreleased`, `openValuePage` | | ✓ | ✓ | ✓ | ✓ | SAM (read grants) |
| `seeSiteCodes` | | ✓ | ✓ | ✓ | ✓ | This service |
| `seeCode`, `seeEmails`, `viewSharing` | ✓ | ✓ | ✓ | | | This service |
| `invite` | any role | facilitator, community scientist, researcher | community scientist, if `facilitatorsInvite` | | | This service and SAM (membership) |
| `manageMembers`, `edit`, `manageSharing` | ✓ | ✓ | | | | This service |
| `manageConsent` | | ✓ | ✓ | | | SAM (`attribute:assert` on `consent`) |
| `manageDeviceAlerts` | | ✓ | ✓ | | | This service |
| `manageTypeAlerts` | | ✓ | | | | This service |
| `manageDevices` | ✓ | ✓ | if `facilitatorsManageDevices` | | | SAM (`device:register`, `device:maintain`) |
| `recordReadings` | see below | ✓ | ✓ | ✓ | | SAM (submit grant) |
| `releaseData` | | ✓ | if `releasers` is `head_and_facilitators` | | | SAM (`dataset:publish`) |

- The front end's grid lets a non-member admin record. Here an admin records, uploads and reads readings only as a member of the classroom, since the admins group holds no read or submit grant. This is an open contract question with the front end.
- A permission marked "This service" is checked by the module before anything is queued or relayed. One marked SAM is also checked here first, so the user gets a clear message, and SAM's refusal is the final word.

### Classrooms in SAM

```
tenant root space ─── admins group: device:register, device:maintain, dataset:set_policy
   │
   └── classroom space (one per classroom)
         ├── heads group
         ├── facilitators group
         ├── community scientists group
         ├── researchers group
         ├── anonymous grants (when shown publicly)
         └── lab spaces, each with a site and devices
```

- A classroom is a SAM space under the tenant root, created with its four role groups and their grants by this service through the outbox.
- Each group holds its role's grants from the grant templates in `internal/roles`, issued by this service from its tenant grant, whose depth is four. Actions that reach different depths are separate grants, and a grant with a rule is one of its own that hands nothing on.
  - **Heads:** read, submit, release and device management, in one grant of depth two that delegates everything the other three groups hold under any sharing settings.
  - **Facilitators:** read and submit, with device management and release while the sharing settings allow them. While `facilitatorsInvite` is on, the same grant has depth one and delegates what community scientists hold; off, it hands nothing on.
  - **Community scientists:** read and submit, in one grant. It carries no consent rule: SAM refuses a minor's submission without consent itself, so withdrawn consent stops new submissions inside SAM while reading keeps working.
  - **Researchers:** read, in a grant of depth one, more than facilitators hand on, so only heads add researchers although their reading is a community scientist's. Its `subject_types` rule names only anonymous, which joins no group and is granted to only by an issuer holding `dataset:publish`, so it hands nothing on.
  - SAM so allows exactly what the grid does, membership included: a role adds members to a group only where the grid lets it invite.
  - **Export:** every group that reads holds `export:create` in a grant of its own, limited by `export_to` to this service, so whoever can see the raw table can download it, through this service only. The grid gains no column for it.
  - **Consent:** `attribute:assert`, limited to `consent` by `attribute_names`, is in separate `head-consent` and `facilitator-consent` grants.
  - Submitting needs `device:connect` beside `observation:submit`, since relayed device readings need both.
  - The grants that hand others theirs carry no rules, because SAM lets a person add members to a group only if their delegating chain carries no rule the group's grants lack.
- Sharing settings that change a role's rights, such as whether facilitators manage devices or release, re-issue that group's grant.
- **Platform admins** are one group at the tenant root. It holds `device:register` and `device:maintain`, so admins set up any classroom's devices as themselves, and `dataset:set_policy`, so they set a classroom's terms before it has a head. It holds no read grant and never `dataset:publish`: an admin who needs a classroom's unreleased data is invited as a member.
- **Creating classrooms.** Platform admins create classrooms, directly or by accepting a host application, naming the head by email, who is invited as its head. Each gets a code such as `DGC-HG2W5K`, which never changes and is shown only to admins, heads and facilitators.
- SAM gives spaces and groups no name, so each classroom records its space's ID and each role group's ID once SAM confirms them.

### Membership

Membership is group membership. Joining, changing role and leaving change the member's group only.

A change is sent to SAM as the person whose decision it carries out, or not at all:

| Changer | May change | Sent as |
|---|---|---|
| Head | Any role but head | The head |
| Facilitator | Add or remove community scientists | The facilitator |
| Platform admin with no role in the classroom | Any role, heads included | This service |
| Anyone else | Nothing | Refused with 403 before anything is queued |

- Taking someone out of their old role needs the same rights as putting them in the new one.
- Making a head is a platform admin's task.
- A member may leave, but a head; a facilitator may remove the community scientists they may invite. Each is sent as themselves: SAM lets a person remove a member only with the rights adding them would need.
- Accepting an invitation to a role below the one a member holds keeps their role.
- A row sent as a head waits until that head's own membership has reached SAM.
- The applier records each step SAM confirms: removal from the old group, then addition to the new one ([SAM integration](sam-integration.md#the-reconciler)).

### Invitations

- `invite` sends one email per address, with a link carrying a random token. Only the token's hash is stored. Inviting an address again replaces its pending invitation, like a resend.
- A new classroom invites the head its platform admin named, as this service.
- An invitation expires after `DGS_INVITATION_TTL`, 14 days by default. An expired one shows as expired and cannot be accepted.
- Accepting needs the token, a signed-in user, and that user's email matching the invitation.
- **An invitation follows its inviter.** When the inviter's role changes, their pending invitations to roles their new role does not hand on are revoked; when they leave, all of them are. Acceptance is sent as the inviter. If the inviter is gone, or does not hold a role that hands the invitation's role on, the invitation answers 404. It is never sent as this service instead, except for an invitation from a platform admin, whose changes go as this service.
- `listMyInvitations` lists pending invitations to the signed-in user's email, newest first. `getInvitation` is public, so the link shows what it points to before sign-in.
- Tokens are returned to the client only in demo mode ([Accounts and sign-in](accounts-and-sign-in.md#environments)).

### Guardian consent

A participant under 18 is a minor. Consent is `pending` until a guardian answers, then `given` or `withdrawn`.

| Way | Flow | Asserted to SAM as |
|---|---|---|
| By link | The minor names a guardian with an email or phone (`requestGuardianConsent`). The guardian gets a link, opens it without signing in, and gives or withdraws consent (`consent.get`, `consent.decide`). Asking again sends a new link, the old one stops working, and consent is pending until answered | This service, as policy (`minor`, `consent`) |
| On paper | The minor is marked (`markMinor`). A head or facilitator prints the sheet (`consentSheet`), collects it signed, and records it (`recordPaperConsent`): given or withdrawn, the guardian's name, and the day on the sheet, a real date not after today | The person who checked the sheet, scoped to the classroom's space; this service's policy assertion follows |

- The record names who checked a paper sheet, and when.
- `listConsent` shows the classroom's minors and their consent to its head and facilitators only. Platform admins are not at the classroom and do not see it.
- A minor without given consent is refused every submission, here and by SAM ([Labs and readings](labs-and-readings.md#relaying-readings)).
- The guardian's contact is stored as a lower-case email or an E.164 number.

**The consent text.** What a guardian agrees to is kept here, not in the front end, so every consent can name the words it was given against.

- **Versions.** The text is kept in `consent_texts`: a version number, a language, and the sheet's parts (title, introduction, sections, the two choices, the contributor and signature lines, the note for the classroom), with `{classroom}`, `{institution}` and `{decimals}` where each classroom's details go. Platform admins publish a version in every language at once, from the admin page or with `dgs consent publish`, and it is never changed or deleted ([Platform configuration](registry-and-content.md#platform-configuration)). The latest published version is the current one.
- **Shown from here.** The printed sheet (`consentSheet`) and the guardian's link page (`consent.get`) carry the current version's words in the reader's language, filled in here for the classroom, with the version number. The front end lays them out and adds nothing to them. The printed sheet shows its version.
- **Recorded with each consent.** A link decision (`consent.decide`) names the version the guardian was shown; a decision against a version that is no longer current is `409 consent_text_changed`, and the page shows the new words. A paper record names the version printed on the sheet, which must be a published one. Each record keeps the version and the rounding the text promised.
- **The promise holds.** A minor's positions are rounded to the coarser of the `privacy` setting and the rounding their consent's text stated, so tightening the setting applies to everyone at once, and loosening it never goes past what a guardian agreed to.
- **Words someone only reads stay in the front end:** the site's pages and policies, and the hosting and join wording ([Platform configuration](registry-and-content.md#platform-configuration)).

### Host applications

- Anyone may apply to host a Global Classroom, without signing in. The readiness questions are the host questions platform admins keep ([Platform configuration](registry-and-content.md#platform-configuration)).
- The application is checked with the same rules as the front end's `validateHostApplication`, and only the answers to the configured questions are kept.
- The applicant gets a status link by email, whose token is stored as its hash.
- Platform admins list applications, newest first, and accept or decline a submitted one. Accepting creates the classroom.

## Alternatives considered

- **Grants per person.** A departure would void the grants that person issued, and a sharing change would touch every member's grant. Rejected for grants held by role groups.
- **Sending every membership change as this service.** A head could make heads, and SAM's audit would name the service. Rejected: as the person, or not at all.
- **One grant per role carrying every action, consent included.** The `attribute_names` rule on it would stop heads and facilitators adding members to groups whose grants lack that rule. Rejected for separate consent grants.
- **Platform admins holding `dataset:publish`.** An admin could make public data they cannot read, which the grid gives to heads and facilitators only. Rejected.
- **Returning link tokens to the client.** Anyone who saw the response could act as the recipient. Rejected outside demo mode.
- **Invitations without expiry or bound to this service's authority.** They would outlive the right that issued them. Rejected.
- **The consent text in the front end.** A release would change what guardians agree to, and no record could say which words a consent was given against. Rejected for versions kept here.
- **Rounding to the current setting only.** Loosening `privacy` would show a minor's position more precisely than their guardian agreed to. Rejected: the coarser of the two applies.

## Risks

| Risk | Mitigation |
|---|---|
| A membership changed in SAM by hand is not repaired | Only this service changes classroom groups; SAM's audit shows any other change |
| An inviter's demotion voids invitations already sent | The invitee sees that the invitation is not valid, and is invited again |
| A classroom set up by an admin shows nothing publicly until it has a head or facilitator to release | Expected: release needs someone who can read the data |
| A forged paper sheet | The record names who checked it; SAM's audit names them too |
| A guardian decides on words that changed while the page was open | The decision names the version shown, and a stale one is refused |
| Platform admins collude to read data | Caught by reviewing the audit, not prevented ([SAM integration](sam-integration.md#who-acts-in-sam)) |

## Testing approach

- **Unit, `roles`:** every cell of the grid, written here; the grant templates, checking that for every grant a group holds, the role that adds members holds one grant that can hand it on, with depth to spare and no rule the held grant lacks.
- **Parity:** every cell the grid marks as SAM's matches SAM's `authz/check` for each role, export included, against a real SAM with the members' `minor=false` asserted. Adding members in SAM succeeds exactly where the grid's `invite` says, and a researcher hands nothing on.
- **Unit, `classrooms`:** who a change is sent as, invitation expiry and acceptance, paper-consent dates, filling in a consent text, a minor's rounding against their consent's.
- **Integration:** joining and leaving change membership only; an expired or orphaned invitation is refused; a minor without consent is refused by SAM; a consent records its text version; a decision on a stale version is refused; a published version cannot be changed; an approval becomes a classroom.

## Open questions

- Whether heads may create classrooms (a contract question with the front end).
- Whether a non-member admin may record or see unreleased readings, as the mock allows (a contract question with the front end).
- Whether guardian links should expire, and whether a guardian's identity needs any check beyond holding the link.
- At what age consent is needed in each pilot country.
- Whether a new consent version asks guardians who already consented to consent again, and whether minors keep contributing meanwhile, decided with the data-protection review.
