# Dashboards and public site

How the pages that show data are computed from SAM, how alert levels work, how data is shared and released, and what visitors see. The front end's `LabService`, `ExportService`, `PublicService` and the sharing calls of `ClassroomService` are the contract.

## Context

The mock computes every view in the browser from its own copy of every reading, and decides what is public by comparing dates with a cutoff. This service keeps no copy of readings ([Labs and readings](labs-and-readings.md)), and SAM must enforce what visitors may read. That leaves these questions:

- where alert levels and their evaluation live;
- what views are computed from;
- how the raw table pages and sorts;
- how "show public", automatic review and the data terms reach SAM;
- what a release is, and who makes it;
- how public pages name contributors and devices when SAM strips identities;
- where the CSV and photos come from.

## Goals

- What a member sees and what a visitor sees come from the same code. Only SAM's answer differs.
- Visitors read exactly what SAM allows them: released records, with positions generalised and people stripped.
- Public pages name only people who asked for credit and chose a public profile.
- Views keep the front end's rounding and wording.

## Non-goals

- Storing or pushing alerts.
- A local copy of readings for faster pages.
- Hiding devices from visitors. No classroom asks for it; SAM's `strip_device` would do it.

## Design

### Reads from SAM

- Views are computed from bounded history, read from SAM as the viewer (anonymous for a visitor), so SAM's read rules decide what is in it.

  | View | Window |
  |---|---|
  | Station page | 62 days |
  | Parameter page | 92 days |
  | Home page | 8 days |

- Each read is capped at 50,000 records. At one reading a minute a device sends about 45,000 readings a month, so the bounds suit the pilot. At larger scale the station and parameter pages move to SAM's aggregates and latest values; no view logic changes shape.
- Every summary asked of SAM carries a start, the start of time when the page has none, so SAM can answer from its hourly rollups.
- Derived values (heat index, feels-like, dew point, wind chill) come from SAM's reading-scoped derivations.
- The view logic lives in `internal/views`, carried over from the front end with its rounding and wording: rain totals, the seven-day strip, sparklines, trends, the spread between sources against tolerance, month completeness, the last 24 hours and device summaries.

### Current conditions and device readings

- Both are read from SAM as the viewer, after their membership here allows it, since SAM reflects a removal only once the outbox delivers it.
- `currentConditions` gives the latest reading per device, only for classrooms the user is a member of. Nobody sees conditions from other classrooms' devices, platform admins included. It reads SAM's latest values (`GET /v1/observations:latest`) for each classroom's devices and keeps, for each device, the values of its newest reading.
- `deviceReadings` gives one device's readings, newest first, 100 by default and 1,000 at most, for anyone who can view its classroom, as SAM lets them read, from SAM's readings list (`GET /v1/observations:readings`), a page at a time. A removed device keeps its history.
- **Values** come back keyed by parameter in each parameter's own unit, as SAM stores them and the front end shows them. A position SAM took from the lab's site (`location_from_site`) is returned as the lab's, with `locationSource: 'lab'`.
- **Who recorded it** comes from `relayed_records`: the recorder's participant ID, the time this service received it, and the name of whoever asked for credit, never a minor's.

### Alert levels

Alert levels and their evaluation live in this service. SAM stores readings and knows nothing of thresholds.

```
for each parameter of a device, the first layer with rules is in force:
  1. the device's own rules            set by the head or a facilitator
  2. the classroom's rules for its type  set by the head
  3. the platform defaults               set by platform admins (registry)
```

- The first change in a layer copies the levels below it for that parameter, so editing one level never drops the others. `resetAlerts` drops a layer's rules for a parameter, back to the next layer down.
- An alert is evaluated against the latest reading when it is read: `currentConditions` carries, for each device, one alert per parameter its newest reading trips, the most severe of the enabled levels, stating the value against the level and whose level it is. Nothing is stored or sent, so alerts follow edits at once.
- Rules are kept in the parameter's own unit, the unit SAM stores values in, so values are compared as SAM returns them.
- A rule stays on its parameter; deleting a device's or type's rule leaves the next layer for that parameter once none is left, and a platform default is never deleted, only turned off.
- Rules live in `alert_rules` with who changed them last; defaults in `alert_defaults` ([Registry and content](registry-and-content.md#alert-defaults)). Layering, the copy on first change and the wording follow the front end's `alerts.ts`, whose tests the service's own repeat.

### The raw table

- `rawValues` is read from SAM as the member, once their membership here allows it; a platform admin who is not a member, like anyone else outside the classroom, gets 404.
- Rows come from SAM's observation list (`GET /v1/observations`), paged by offset, up to 500 per page, read 200 at a time. SAM takes an offset of at most 10,000 rows, so a page past it is refused on `page`.
- The total and the filter options come from SAM's summary (`GET /v1/observations:summary`). The options narrow in order, as the front end's mock does: the sites holding values, the sources of the site picked, the parameters of the source picked, and the days the other filters match, whatever days are asked.
- **Sorts.** Time sorts by `observed_at`, newest first by default; parameter by `parameter_id`, source by `device_id` and site by `space_id`, ascending by default. SAM sorts parameter, source and site by their IDs, not by the labels shown, so a sort by those columns groups rows but does not order them alphabetically.
- **Filters.** A site is its lab's space; a source is a device's SAM ID, or `field` for field observations; the `from` and `to` days become SAM's `from` and the day after `to`, since SAM's range excludes its end.
- **Rows** name their source and site, a position SAM took from the site as the lab's, rounded as the classroom's terms say, and the recorder's participant ID from `relayed_records`. A removed device's values stay.

### Member views of station and parameter pages

The station and parameter pages a member sees are the public pages read as the member: the classroom's own members also get values still in review, and the last sync of any device.

### Sharing and release

A classroom's sharing settings:

| Setting | Meaning |
|---|---|
| `showPublic` | Visitors see released data |
| `review` | `manual`: released by hand; `auto`: also released by age |
| `autoReleaseDays` | With automatic review, readings older than this release themselves; at most 30 |
| `releasers` | The head alone, or the head and facilitators |
| `facilitatorsInvite`, `facilitatorsManageDevices` | What facilitators may do |

**The public view is anonymous grants.** While a classroom shows its data publicly, SAM's anonymous principal holds `observation:read` on its space for published records only, with `generalise_location` and `strip_identity`.

- The grid is about 1 km, from the classroom's coordinate decimals, never finer than two.
- Automatic review adds a second grant with an `embargo` of the review period.
- Grants to anonymous only read; this service's tenant grant holds `dataset:publish`, which SAM requires of their issuer.
- Each grant is named after what it allows, `public:published` and `public:embargo`, and labelled `classroom-public`, so a change of settings revokes the old grant and issues the new one. Turning "show public" off revokes both, queued as a change that takes access away.
- They follow this service's rules, not a person's rights, so they are sent as this service, by the classroom kind, and the reconciler's `public` check repairs them ([SAM integration](sam-integration.md#the-reconciler)).
- A role's grant is replaced the same way when a setting changes what it allows: `releasers` and `facilitatorsManageDevices` change the facilitators' uses, and `facilitatorsInvite` what they hand on. A change that narrows a role is queued as taking access away.

**Data terms are the space's data policy.** Licence, attribution and restrictions travel with every result SAM returns. The policy (kind `classroom_policy`) is sent as the head who changed the terms, whose grant holds `dataset:set_policy`, or, for a classroom with no head yet, as the platform admin who created it. It waits until SAM holds that person's rights, names the policy version SAM holds, recorded in `sam_policy_version`, and reads SAM's version back when another was set meanwhile. A licence address must be https, as SAM takes.

**A release is a SAM publish up to a time, as the person releasing.**

- `releaseData(through)` stores the cutoff in `released_through` and queues a publish (kind `release`) through the outbox, once SAM holds the person's membership. SAM checks the person's `dataset:publish` and records them as publisher. A `through` later than now is taken as now, and the cutoff never moves back: a release at or before it changes and queues nothing.
- Its idempotency key names the release, so a retry after a lost answer gets the first answer back and nothing is published twice.
- Platform admins do not release.
- `releaseReview` counts what waits: SAM's summary of unpublished values observed after the cutoff, as the viewer may read them, by source. Values are counted, not readings, so a weather station reading counts once per value.
- A reading that arrives after a release with an older date stays unpublished until the next release.

**Automatic review and the embargo.** SAM's embargo runs from when it received a record; the front end's automatic cutoff runs from when it was observed. A reading observed long ago but received recently shows as released on members' pages before SAM lets visitors read it. SAM's answer is the one visitors get.

### Public pages

| Page | Shows |
|---|---|
| Home | The stages and regions, and every classroom with its stage and core readings |
| Station | A classroom's latest released value of each parameter, with the device each came from |
| Parameter | One parameter's registry entry and released values over time |
| Classroom profile | Who they are, what they study, their kit, and contributors who asked for credit |
| Contributor | A contributor's released values and classrooms |

- Pages are read as SAM's anonymous principal. View models are cached for a short time.
- Visitors see only what has been released. Without an anonymous grant on a classroom's space, SAM gives a visitor nothing: public pages show the classroom without values, and a parameter page is not found.
- **Devices.** Station tiles use the `device_id` SAM returns to visitors: `strip_identity` removes people and keeps devices. Readings a device sends straight to SAM show their device too.

### Contributors and credit

- A released record is linked to a person only through the credit they asked for (`relayed_records.credit_user_id`), never through who recorded it.
- A minor is never listed or given a page, whatever their settings.
- A contributor page exists only for a non-minor with a public profile. Every other participant ID answers 404, the same as an unknown one, so pages cannot be found by counting.
- No email is public: the profile on a contributor page has `email: null`.

### CSV export

- `rawCsv` pages through the same SAM list as the raw table, as the member, up to 100,000 rows, a thousand at a time.
- This service adds what SAM does not know: labels, device, site, classroom and participant link, in the front end's columns.
- Every row carries the classroom's licence, attribution and restrictions.

### Photos

- An institution's photo is set by its head (`setInstitutionPhoto`) and kept behind a store interface: files on a volume in development, object storage when deployed.
- Names are content hashes, so a file never changes and is cached for good. Files are served at `/api/media/{name}`; the `media` table records what each belongs to.
- Uploads are bounded in size and checked to be images.

## Alternatives considered

- **Computing views from a local copy of readings.** A second copy to reconcile, and SAM's read rules would not apply. Rejected.
- **Deciding what is public by a cutoff compared here, as the mock does.** SAM would not enforce it, and a bug here would publish unreleased data. Rejected for anonymous grants on published records.
- **The admins group holding `dataset:publish`.** Admins could make public data they cannot read. Rejected.
- **A new idempotency key per release attempt.** A retry after a lost answer would publish twice. Rejected.
- **Listing everyone who recorded a released record.** It would name people who never asked for credit, and a page for every participant ID would let IDs be enumerated. Rejected for credit only.
- **Keeping each record's device here for public tiles.** It would cover only relayed readings. Rejected: SAM keeps devices for visitors.
- **Reading aggregates from the start.** More calls and shapes to build for the pilot's scale. Deferred: bounded history first, aggregates when the bound bites.

## Risks

| Risk | Mitigation |
|---|---|
| Pages outgrow the 50,000-record cap | Move to SAM's aggregates and latest values; the view logic keeps its shape |
| Members and visitors disagree about what automatic review has released | SAM's answer is final for visitors; the difference is short and documented |
| Cached public pages show a revoked release briefly | Short cache lifetime |
| Contributor pages identify a minor | Minors are never listed; positions are generalised; identities are stripped |

## Testing approach

- **Unit, `views`:** rain totals, spread, completeness, trends, the last 24 hours, device summaries, with the front end's rounding.
- **Unit, `labs`:** alert layering and the copy on first change, CSV shaping.
- **Unit, `publicsite`:** the cutoff, projects, sessions, the seven-day strip, latest values.
- **Integration:** visitors see released data only; turning "show public" off hides it; a retried release publishes once; a contributor without a public profile answers 404; the CSV downloads with terms on every row; sorting by parameter, source or site pages correctly.
- **End to end:** the front end's HTTP client runs through release, the visitor pages and the CSV export on the seeded demo history.

## Open questions

- How long public view models are cached.
- Whether alerts should ever be pushed, which would need this service to watch readings.
- Whether a removed device's readings stay on public pages (a contract question with the front end).
