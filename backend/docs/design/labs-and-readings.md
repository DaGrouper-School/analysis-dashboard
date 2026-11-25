# Labs and readings

How labs and devices are kept in SAM, and how readings and observations reach it: device uploads, manual observations and offline batches. The front end's `LabService` is the contract.

## Context

A lab is a place where a classroom measures: a school garden, a reef, a river bank. It has a site questionnaire and a location, and devices typed by the registry (weather stations, water probes, rain gauges read by hand). SAM is the system of record for observations and holds every right a reading needs. Five questions follow:

- what a lab and a device are in SAM;
- who acts in SAM when they change;
- how readings reach SAM;
- what this service keeps about them;
- how a retried offline batch avoids recording twice.

Data can arrive by three routes, and all of them end in SAM's validation, flags and storage:

| Route | Path | Attestation |
|---|---|---|
| Sensor direct | The device or its host computer posts signed readings to SAM | `device` or `gateway` |
| Through the dashboard | A member's tablet reads the device, and this service relays the readings on the member's behalf | `user` |
| Manual entry | An observer fills a template: a rain gauge, a species sighting or count | `user` |

## Goals

- Labs and devices exist in SAM with the structure SAM's rights need.
- SAM's answer to a submission is the user's answer: its refusal is the person's refusal.
- No copy of a reading is kept here, so there is nothing to reconcile.
- A retried offline batch stores each record once, even if an answer was lost.
- A minor's position never pins down where they were.

## Non-goals

- Real pairing of devices in the browser and gateway signing. The front end's pairing stays simulated.
- Device keys for the classroom kits, which have none yet.
- Queuing submissions in the outbox. The offline path is the one built for retries.

## Design

### Labs

- A lab is a SAM space under its classroom's space, with a SAM site at the lab. The site's code is the classroom's code and the lab's number in it, such as `ADD-01`, and its geometry is the lab's point.
- The lab's name, place name, site questionnaire and the classroom's notes on each value stay here.
- A lab follows the classroom's structure, so this service creates and moves its space and site through the outbox, as it does the classroom's space.

### Devices

- A device is a SAM device principal in its lab's space, registered and removed as the person who did it. SAM checks that they hold `device:register` or `device:maintain` ([Classrooms and permissions](classrooms-and-permissions.md#classrooms-in-sam)).
- Devices are registered with `unsigned_readings: flag`, since the kits have no keys: SAM stores their readings and flags them.
- The display name and hardware identifier stay here. The identifier is unique among a classroom's live devices.
- A registration SAM refuses is retried for a while, because the person's group membership may still be on its way. After that the row is held.
- A platform admin with no role in a classroom can register its devices but cannot record or read its readings.

### Relaying readings

Readings and observations are relayed to SAM synchronously, as the person recording.

1. **Check here first.** The person must hold `recordReadings` and be a member. A minor needs given consent: this service checks its own record, since SAM reflects a withdrawal only once the outbox delivers it. The refusal names the reason ("Your guardian needs to give consent…").
2. **Check readiness.** A lab or device not yet in SAM answers `409 conflict`, "still being set up".
3. **Convert at the edge.** The front end sends values by parameter, in display units and with category labels. They are sent as the device's keys or the template's fields, in their own units and values, using the registry snapshot, and converted back with the same snapshot when read.
4. **Place it.** A GPS fix is sent with the reading. For a minor it is first rounded to the coarser of the `privacy` setting and the rounding their consent's text stated ([Guardian consent](classrooms-and-permissions.md#guardian-consent)). Without a fix, SAM uses the lab's site, and the front end shows `locationSource: 'lab'`.
5. **Send it.** Device uploads and device-template entries go to SAM's device readings; form templates go to SAM's template submissions in the lab's space. Every reading of a multi-reading upload is sent and stored.
6. **Return SAM's answer** in the front end's shape.

This service keeps one row per relayed record in `relayed_records`: the form it came through, who recorded it, and who asked for credit. A name appears with a record only with credit, and never a minor's.

### Manual observations

- `observationTargets` lists the labs the person may record in, with their devices.
- `submitObservation` takes a template's fields. The template's checks run again here, then the record goes to SAM: under the device for a device reading, under the lab for a field observation.
- Templates are SAM registry entries, presented through the overlay, plus a plain one for each device type without one ([Registry and content](registry-and-content.md#observation-templates)).

### Offline sync

The front end keeps observations made offline on the device and sends them as one batch to `POST /observations/batch`. Each record gets its own result.

- **Idempotent twice over.** A record's SAM ID is derived from the person and the client's ID (a UUID v5), so SAM deduplicates a retry even if this service never saw the answer. The answer is also stored by the client's ID in `synced_records`, so a retry gets it back without asking SAM again. A record stored before comes back as a duplicate.
- **The device clock.** The offset is measured from the batch's `sentAt`. Records off by more than five minutes are flagged, not refused.

### Records made elsewhere

`ImportReadings` and `ImportObservation` store records made outside the dashboard under IDs the importer fixes. The demo data tool uses them; nothing in them is specific to it.

## Alternatives considered

- **Keeping a copy of each reading here.** It would need reconciling with SAM, and SAM's read rules (licences, location, identity) would not apply to pages built from it. Rejected.
- **Queuing submissions in the outbox.** A user needs SAM's per-record validation as the answer, and a queued reading could fail long after the user left. Rejected: the offline batch is the retry path.
- **Random SAM IDs for offline records.** A lost answer followed by a retry would store a record twice. Rejected for IDs derived from the person and the client's ID.
- **Refusing records with a bad device clock.** Readings from a tablet with a wrong clock would be lost. Rejected: they are flagged.
- **Platform admins reading every classroom.** It would give the programme's staff every minor's data. Rejected: admins manage, and read only as members.

## Risks

| Risk | Mitigation |
|---|---|
| A SAM outage fails uploads | The client retries; offline sync is built for it |
| Unsigned readings | Flagged by SAM; signed readings come with device keys |
| Each page view is a SAM call | Bounded windows, and SAM's aggregates at scale ([Dashboards and public site](dashboards-and-public-site.md#reads-from-sam)) |
| A registration waits on a membership still on its way | Retried for a while, then held |

## Testing approach

- **Unit:** unit conversion both ways, the minor's rounding, the clock-offset flag, the UUID v5 derivation, the consent refusal messages.
- **Integration:** every reading of a multi-reading upload is stored; a retried batch stores once and returns duplicates; a minor without consent is refused here and by SAM; a lab still being set up answers 409; a device's key and policy round-trip.

## Open questions

- How sensors finally reach SAM. The current agreement is direct sending, and SAM's signed envelope works by either path; the relay cannot alter a signed reading.
- Whether a removed device's readings stay in exports and on public pages (a contract question with the front end).
- One rule for future-dated observations across upload, submit and batch (a contract question with the front end).
- The `credit` flag, which the UI always sends as false (a contract question with the front end).
