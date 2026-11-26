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

- Views are computed from SAM's aggregates, latest values and summaries, read as the viewer (anonymous for a visitor), so SAM's read rules decide what is in them. Nothing is copied here.
- A page's values span a window up to its latest value, in whole UTC days as the front end counts them: a parameter page the 91 days of its longest range, the season; the home page the 7 days of its rain strip. Numbers older than a day come as each hour's last value, from hourly aggregates SAM answers from its rollups; the last day's readings come one by one, so a day's chart shows every reading. Values that are not numbers come one by one, up to 5,000.
- An aggregate is read in spans short enough to stay under SAM's 10,000 groups: a span holds 10,000 hours divided by the sources and values read.
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

- The public routes read a session when there is one. A caller this service records as a member of the classroom reads SAM as themselves; anyone else, a platform admin who is not a member included, reads as anonymous, as does a member SAM does not hold yet, until the outbox brings them.
- A visitor reads released values only, and a classroom that does not show its data publicly answers 404 on its parameter pages, whatever SAM still lets anonymous read. Its members still read it.
- `inReview` is a member's alone: whether values observed after the cutoff are unpublished. A visitor's is false, as visitors learn nothing of what waits.
- A station page reads the latest values, each source's values by day over the 31 days up to the newest (the month of air temperature, the week's rain and each device's seven days), and by hour over the last day. The core detail is computed from released values only, for members too, as the front end's `stationCore`; members read the released days apart.
- "What's happening now?" shows the tiles of the `now_tiles` setting, each its first value with data; its last 24 hours and its trend come from the hourly values, so an hour shows its last reading and a trend is fitted through those.
- Month completeness counts the hours with a reading of each value the protocol expects, from the first of the month to the core day, as the reader reads them: a visitor counts released values only, and learns nothing of what waits.
- Every device the classroom has, removed ones aside, is listed with its tiles and cards from its newest reading the reader may see, and with no data when there is none, so the page keeps its layout. A visitor to a classroom that does not show its data publicly sees its devices this way.
- A parameter page offers to compare with every other classroom that shows its data and whose released values include the parameter, as anonymous reads them, each answer kept for a minute.

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

- Pages are read as SAM's anonymous principal. View models are cached for a short time: a classroom's core readings for a minute.
- **The settings here decide first.** A classroom that does not show its data publicly is listed with no values, whatever SAM still lets anonymous read before the revocation reaches it. The setting is read on every request, never cached.
- Visitors see only what has been released. Without an anonymous grant on a classroom's space, SAM gives a visitor nothing: public pages show the classroom without values, and a parameter page is not found.
- **Review on public pages** is released, or not released while nothing is. A visitor reads released data only, so whether newer data waits is not theirs to know; members' own views show it.
- **Site content.** `publicSite.content` and the home page's content are the stages and regions from platform configuration; the site's words stay in the front end (the contract's addendum).
- **The home page's core readings** come from SAM's latest values (today's day, air temperature and last sync) and an aggregate of the rain values by UTC day, source and parameter over the week up to the newest, computed in `internal/views` as the front end's `coreReadings` does: the reference gauge's total over another source's, the week summed over the days with a total, missing values null. The units come from the registry. The host is named once it has signed the host agreement, and positions are rounded to the classroom's coordinate decimals, never finer than two.
- **Devices.** Station tiles use the `device_id` SAM returns to visitors: `strip_identity` removes people and keeps devices. Readings a device sends straight to SAM show their device too.

### Contributors and credit

- A released record is linked to a person only through the credit they asked for (`relayed_records.credit_user_id`), never through who recorded it.
- Which credited values are released is asked of SAM, as a visitor, filtered by the credited readings (`reading_id`, a hundred a read), never worked out here by comparing times with the cutoff: a value sent after a release, observed before it, waits for the next one and is never shown early.
- A classroom profile lists each person credited on a value the public may read, named when they turned on their public profile and by participant ID otherwise, named ones first. A classroom that does not show its data publicly lists none, and drops out of its contributors' pages at once.
- A contributor page holds how many credited values the public may read, the twenty newest of them, and the classrooms they are in.
- A minor is never listed or given a page, whatever their settings, and nor is someone not yet known to be an adult.
- A contributor page exists only for a non-minor with a public profile. Every other participant ID answers 404, the same as an unknown one, so pages cannot be found by counting.
- No email is public: the profile on a contributor page has `email: null`.

### CSV export

- `rawCsv` reads SAM's export (`GET /v1/observations:export`) as the member, with the raw table's filters, under the member's `export:create` grant, which `export_to` limits to this service. It reads JSON lines and completes them 500 at a time.
- This service adds what SAM does not know: labels, device, site, classroom and participant link, in the front end's columns, then `licences`, those the reader's grants attach. The participant link is the contributor page on the front end that asked.
- Every row carries the licence, attribution and restrictions of the data policy SAM puts on it; until the classroom's terms reach SAM, the classroom's own.
- A removed device's released readings stay in it, as in the raw table.
- The whole file goes in the answer, `{filename, content, rows}`, so an export holds at most 100,000 values; a larger one is refused, to be narrowed. Rows are written through `RowWriter`, so a streamed download can replace the body without changing how rows are made.

### Photos

- An institution's photo is set by its head (`setInstitutionPhoto`) and kept behind a store interface (`internal/media`): an S3-compatible object store, SeaweedFS's S3 gateway in the compose file until a cloud provider is chosen, or memory in development and test without one.
- Names are content hashes, so a file never changes and is cached for good. Files are served at `/api/media/{name}`, with the type the `media` table records, which also records what each belongs to. The profile names the photo by its full address, this service's public URL, as the front end may run on another origin.
- Uploads are bounded in size, 1.5 MB of image, and must be PNG, JPEG or WebP by their bytes as well as by what they say; anything else is refused on `image`, as is an alt text of fewer than three characters on `alt`.
- Files are served with `nosniff` and a sandboxing content security policy, so a file cannot run as a page.

## Alternatives considered

- **Computing views from a local copy of readings.** A second copy to reconcile, and SAM's read rules would not apply. Rejected.
- **Deciding what is public by a cutoff compared here, as the mock does.** SAM would not enforce it, and a bug here would publish unreleased data. Rejected for anonymous grants on published records.
- **The admins group holding `dataset:publish`.** Admins could make public data they cannot read. Rejected.
- **A new idempotency key per release attempt.** A retry after a lost answer would publish twice. Rejected.
- **Listing everyone who recorded a released record.** It would name people who never asked for credit, and a page for every participant ID would let IDs be enumerated. Rejected for credit only.
- **Keeping each record's device here for public tiles.** It would cover only relayed readings. Rejected: SAM keeps devices for visitors.
- **Bounded history, every reading over each page's window.** At one reading a minute a season is about 130,000 readings a value, too many to read for each page. Rejected for hourly aggregates beyond the last day.

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
