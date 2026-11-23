# Registry and content

How the registry the front end reads is built from SAM's registry and this service's overlay, what platform admins may change, and how configuration that no page writes gets in. The front end's `RegistryService` and `PublicService.content` are the contract.

## Context

SAM's registry is the source for domains, parameters, device types and observation templates: a shared core, plus entries a tenant owns under its namespace (`dgs:`), owned by its root space. SAM's rules limit what a tenant may change:

- domains and units are core only;
- a tenant may add parameters, device types and templates under its namespace;
- a tenant parameter needs one of SAM's domains;
- SAM keeps readings in a parameter's unit, and refuses a unit change while a device type sends another unit or observations use it.

The front end needs more than SAM holds: plain-language labels, icons, colours, chart kinds, explanations, which values show as tiles and cards, display units, noise thresholds and default alert levels. Platform admins change all of this from the admin page, and every change is logged. Platform admin is this service's own role: admins hold no SAM grants, and `registry:manage` on the tenant root belongs to this service's principal.

The public pages also depend on configuration that no API operation writes: the site's content, stages and regions, the core readings' parameters, a new classroom's data terms, privacy rounding, the host application's questions, and each classroom's stage, region and profile fields.

## Goals

- The admin page works on this service, with the mock's behaviour except where SAM's rules forbid it.
- Only SAM's core entries and this tenant's own entries are shown.
- An admin sees their change at once.
- Configuration is checked before it is stored, so a mistake never reaches a page.

## Non-goals

- Changing SAM's core entries.
- Domains or units in SAM owned by this tenant.
- Pages or endpoints that edit operator configuration.

## Design

### Reading the registry

- SAM's effective registry for this tenant is read and cached apart from the overlay. Concurrent misses share one call to SAM. When SAM cannot be reached, the cached copy is served and SAM is asked again after ten seconds.
- The registry the front end reads is SAM's entries merged with the overlay and this tenant's own definitions. Everyone signed in can read it.
- IDs are SAM's, as they are: the front end sees `dgs:soil_ec` and `air_pressure.barometric`.
- `getCalculation` returns a value's formula, inputs and SAM version, for "How is this calculated?".

### The overlay

Presentation stays here, in overlay rows keyed by SAM ID. SAM never sees them:

| Overlay | Examples |
|---|---|
| Text | Label, plain-language description, explanation of how it is calculated |
| Display | Icon, colour, chart kind, display unit, display order, noise threshold |
| Placement | Which domain a value is shown under; which values are tiles and cards |

### Domains

- SAM's domains are shown unless an admin removes them here, which only hides them. Adding one of SAM's codes again shows it again.
- A domain an admin adds is this service's own display group, stored here alone. Its code never changes. It cannot be deleted while a value is shown under it.
- Moving a value changes where it is shown. A move of one of this tenant's own values to one of SAM's domains also reaches SAM.

### This tenant's own entries

- A value new to the registry becomes the parameter `dgs:<id>`; a new device type becomes `dgs:<name>`. The device type maps each value under the key the admin typed, which is the key its devices send.
- Each definition is kept in `registry_own_entries` and sent to SAM through the outbox, as this service, the holder of `registry:manage`. The admin's identity is recorded in this service's change log, not in SAM's audit.
- A device type is sent only after the parameters it maps.
- The registry shows this service's definition until SAM has it.
- When an admin types an ID that is already this tenant's own, it names that value again.

### What SAM's rules refuse

| Change | Answer | Why |
|---|---|---|
| A core device type's values, or whether it is read by hand | 409 | SAM defines them; its label, description and presentation can change |
| A value's unit or kind after it is registered | 400 | SAM keeps readings in the registered unit; an admin adds a new value for another unit |
| A new value without one of SAM's domains | 400 on `values.<i>.domain` | SAM requires a core domain for a tenant parameter. Once added, it can be shown under any display group |

The mock also allows "no domain" and the admins' own domains for a new value; SAM refuses both, so this service cannot follow the mock there.

### Change log

Every change is logged in `registry_changes`, with the summaries the mock writes, the admin's name at the time, newest first. `listChanges` is for platform admins only.

An admin's change recomputes the merged registry from the cached copy of SAM's and the database, without calling SAM. One change runs at a time, against the registry the change before it left.

### Alert defaults

The platform's default alert levels are this service's own (`alert_defaults`), in each parameter's display unit. They are the lowest layer of a device's alert levels ([Dashboards and public site](dashboards-and-public-site.md#alert-levels)).

### Observation templates

Templates are SAM registry entries, presented through the overlay. `listObservationTemplates` adds a plain one for each device type that has none, under the `device:` prefix.

### Operator configuration

No API writes this configuration. It changes rarely, through the people who run the service, and the front end only reads it.

| Where | Keys or fields |
|---|---|
| Content store (`site_config`), one JSON value per key | `site_content` (sections, policies, stages, regions), `core_parameters`, `default_terms`, `privacy` (rounding, including the minors' coordinate decimals), `host_questions` |
| Each classroom | `stageId`, `regionId`, and profile fields such as `hostAgreementSigned`, projects and sessions |

```sh
dgs content keys                           # the keys above
dgs content get site_content > site.json   # edit, then
dgs content put site_content site.json
dgs classroom configure DGC-HG2W5K classroom.json
```

- A classroom file may hold any of `stageId`, `regionId` (null clears it) and `profile` fields. A field left out stays as it is, and profile fields replace their own value only. A stage or region must be one the site content names.
- Every value is checked before it is stored, by `dgs content`, `dgs classroom configure` and `dgs seed` alike: known keys, strict shapes, and the fields each page reads.
- Pages read these values on each request, so a change shows at once.
- What belongs to other modules stays there: a lab's parameter notes are set with labs; templates are registry entries.
- **The seed.** `dgs seed` loads the overlay and the site content from the front end's seed, exported as `seed/frontend-seed.json` by `npm run seed:export` in `frontend/`. It updates the same rows on a re-run and is safe in every environment.

## Alternatives considered

- **Presentation in SAM.** Labels, charts and tiles are DaGrouper's, and a second tenant would not use them unchanged. Rejected for the overlay.
- **Admins holding `registry:manage` themselves.** Platform admin is this service's role, and SAM would see admins as registry authorities for every purpose. Rejected: this service sends the entries and records the admin in its change log.
- **Calling SAM for each admin change.** The page would wait on SAM and fail in an outage. Rejected: the definition is shown at once and sent through the outbox.
- **Endpoints and pages to edit configuration.** The values change rarely and no page needs an editor. Deferred: an editor would write through the same checks.
- **Configuration in environment variables or files read at start-up.** A change would need a restart, and nothing would check the shapes the pages read. Rejected for the checked content store.

## Risks

| Risk | Mitigation |
|---|---|
| SAM refuses a tenant entry | The row is held for an operator; the admin page shows the definition, so an admin may see a value devices cannot yet send |
| SAM's registry is unreachable | The cached copy is served |
| A configuration mistake breaks a page | Checked before it is stored |

## Testing approach

- **Unit:** the merge of SAM's registry with the overlay and own entries; unit conversion; each configuration key's checks.
- **Integration:** only this tenant's and core entries show; a new value reaches SAM as `dgs:<id>`; a core device type's values are refused; a device type waits for its parameters; `dgs content put` refuses a bad shape.

## Open questions

- Whether SAM will allow tenant parameters without a core domain, or tenant-owned domains, so the admin page can match the mock.
- Whether admins will want an editor for site content.
