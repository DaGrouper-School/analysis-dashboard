# Registry and content

How the registry the front end reads is built from SAM's registry and this service's overlay, what platform admins may change, and how they keep the platform's configuration: stages, regions, host questions and settings. The front end's `RegistryService` and `PublicService.content` are the contract.

## Context

SAM's registry is the source for domains, parameters, device types and observation templates: a shared core, plus entries a tenant owns under its namespace (`dgs:`), owned by its root space. SAM's rules limit what a tenant may change:

- units are core only, in one catalogue;
- a tenant may add domains, parameters, device types and templates under its namespace;
- a tenant parameter uses one of the core's domains or one of its own;
- SAM keeps readings in a parameter's unit, and refuses a unit change while a device type sends another unit or observations use it.

The front end needs more than SAM holds: plain-language labels, icons, colours, chart kinds, explanations, which values show as tiles and cards, display units, noise thresholds and default alert levels. Platform admins change all of this from the admin page, and every change is logged. Platform admin is this service's own role: admins hold no SAM grants, and `registry:manage` on the tenant root belongs to this service's principal.

The public pages and some of this service's rules also depend on the platform's configuration: classroom stages and regions, the host application's questions, the core readings' parameters, a new classroom's data terms, privacy rounding, and each classroom's stage, region and profile fields. Classrooms and applications refer to the stages, regions and questions, and the server applies the settings itself, so they are kept here rather than in the front end. The site's own words are not configuration: they live in the front end, with its translations. Words someone agrees to are the exception: the guardian consent text is kept here, in versions, so every consent names the text it was given against ([Guardian consent](classrooms-and-permissions.md#guardian-consent)).

## Goals

- The admin page works on this service, with the mock's behaviour except where SAM's rules forbid it.
- Only SAM's core entries and this tenant's own entries are shown.
- An admin sees their change at once.
- Configuration is checked before it is stored, so a mistake never reaches a page.
- Platform admins keep the configuration from the admin page, and every change is logged.
- Nothing a classroom or an application refers to can be removed from under it.

## Non-goals

- Changing SAM's core entries.
- Units of this tenant's own. SAM's catalogue is the core's.
- The site's words: the home page, sections and policies, the hosting and join wording, and its languages. The front end holds them with its translations, and they change with a release. The consent text is not among them: a guardian agrees to it, so it is kept here.
- Platform configuration in SAM's back office. Stages, regions and questions are DaGrouper's, and a second tenant would not use them.

## Design

### Reading the registry

- This service reads SAM's registry as itself, effective in the tenant's root (`?space_id=` the root): the domains, parameters, device types and templates, each page by page, so only the core's entries and this tenant's show. It also reads the unit catalogue and the derivations. All of these are open to every caller.
- The copy is cached apart from the overlay and served for ten minutes. It is read again sooner once one of this tenant's entries reaches SAM. Concurrent misses share one read. When SAM cannot be reached, the cached copy is served and SAM is asked again after ten seconds.
- The registry the front end reads is SAM's entries merged with the overlay and this tenant's own definitions. Everyone signed in can read it, and it is merged on each request, so an overlay change shows on the next one.
- IDs are SAM's, as they are: the front end sees `dgs:soil_ec` and `barometric_pressure`.
- **Units.** A unit is shown as people read it, `°C` for SAM's `degC`. A value's other units are the catalogue's units of its dimension, each with the factor and offset that convert it to the value's own.
- **Device types** list the values their keys map, in key order. A device type with no sensor lists the values of the templates read off it.
- `getCalculation` returns a value's formula, inputs and SAM version, for "How is this calculated?". A derived value's come from its SAM derivation; a measured or entered one says how its source takes it.

### The overlay

Presentation stays here, in overlay rows keyed by SAM ID. SAM never sees them:

| Overlay | Examples |
|---|---|
| Text | Label, plain-language description, explanation of how it is calculated |
| Display | Icon, colour, chart kind, display unit, display order, noise threshold |
| Placement | Which domain a value is shown under; which values are tiles and cards; a template's domain and the device type it is read off |

There is one table per kind of entry: `registry_domains`, `registry_parameters`, `registry_device_types` and `registry_templates`, in display order. An entry with no row is shown as SAM defines it, after those with one, with a plain icon and colour. A value with no row has the source a device type implies: a device if one with a sensor maps it, a person otherwise.

### Domains

- SAM's domains are shown unless an admin removes them here, which only hides them. Adding one of SAM's codes again shows it again.
- A domain an admin adds is this tenant's own SAM domain, `dgs:<CODE>`, sent through the outbox, so a new value may use it. Its code never changes. It cannot be removed while a value is shown under it or held under it in SAM; removing it removes it from SAM once this tenant's values have moved out of it there.
- Moving a value changes where it is shown. A move of one of this tenant's own values to a domain also reaches SAM, since every domain shown is one of SAM's. A value taken out of every domain is shown under none and stays where it is in SAM.

### This tenant's own entries

- A value new to the registry becomes the parameter `dgs:<id>`; a new device type becomes `dgs:<name>`. The device type maps each value under the key the admin typed, which is the key its devices send.
- A device type read by hand has no sensor, so SAM gives it no keys. Its values become the fields of a template of the same ID, read off it, with the first value required.
- Each definition is kept in `registry_own_entries` and sent to SAM through the outbox, as this service, the holder of `registry:manage`. Each write names the version SAM holds, `0` to create. The admin's identity is recorded in this service's change log, not in SAM's audit.
- A value is sent only after the domain it names, and a device type or template only after the values it maps. Once SAM has an entry, the cached copy of SAM's registry is read again.
- An entry SAM refuses is held for an operator, like any change SAM refuses.
- The registry shows this service's definition until SAM has it.
- When an admin types an ID that is already this tenant's own, it names that value again.

### What SAM's rules refuse

| Change | Answer | Why |
|---|---|---|
| A core device type's values, or whether it is read by hand | 409 | SAM defines them; its label, description and presentation can change |
| Whether one of this tenant's device types is read by hand, once registered | 409 | A device type read by hand has a template instead of keys |
| A value's unit or kind after it is registered | 400 | SAM keeps readings in the registered unit; an admin adds a new value for another unit |
| A new value without a domain | 400 on `values.<i>.domain` | SAM files every parameter under a domain. Once added, it can be shown under no domain |
| A value SAM derives explained as measured, or with other inputs; a value SAM does not derive explained as derived | 400 | SAM's derivation is the calculation; an admin rewords its formula only |

The mock also allows "no domain" for a new value; SAM refuses it, so this service cannot follow the mock there.

### Explanations

An admin sets what a value means and how it is calculated. For a value SAM derives, the method stays derived and the inputs SAM's, and the formula may be reworded. For any other value, the method is measured or entered by hand, with no inputs, and the wording replaces that of its source.

### Change log

Every change is logged in `registry_changes`, with the summaries the mock writes, the admin's name at the time, newest first. `listChanges` is for platform admins only.

An admin's change recomputes the merged registry from the cached copy of SAM's and the database, without calling SAM. One change runs at a time, against the registry the change before it left.

### Alert defaults

The platform's default alert levels are this service's own (`alert_defaults`), in each parameter's display unit. They are the lowest layer of a device's alert levels ([Dashboards and public site](dashboards-and-public-site.md#alert-levels)). Everyone signed in reads them; platform admins add, change, turn off and remove them, each a number value of the registry within its range, and each logged. `dgs seed` loads the front end's defaults.

### Observation templates

Templates are SAM registry entries, presented through the overlay. `listObservationTemplates` adds a plain one for each device type that has none, under the `device:` prefix.

### Platform configuration

| What | Kept as | Owned by | Read by |
|---|---|---|---|
| Stages | Rows in `stages`: ID, label, description, colour, position | Classrooms | Classroom profiles, the directory and the map. A classroom's `stageId` refers to one |
| Regions | Rows in `regions`: ID, label, position | Classrooms | The directory. A classroom's `regionId` refers to one |
| Host questions | Rows in `host_questions`: ID, label, help, kind, required, position, when retired | Hosting | The host application. An application's answers refer to them |
| Settings | One checked value per key in `platform_settings`: `core_parameters`, `default_terms`, `privacy`, `future_observations` | `internal/settings` | Station pages and the network map; classroom creation; readings from minors |
| Each classroom | `stageId`, `regionId`, and profile fields such as `hostAgreementSigned`, projects and sessions | Classrooms | Its profile |
| Consent text | Published versions in `consent_texts`, one row per version and language, never changed | Classrooms | The printed consent sheet and the guardian's link page |

All of it is kept from the admin page in the dashboard, and the same changes can be made from a terminal.

- **Lists are rows.** A stage, region or question is a row that other rows point to, through a foreign key. A stage or region in use cannot be deleted (`409 conflict`), and a classroom cannot name one that does not exist. A question that has answers is retired rather than deleted, so earlier applications keep their meaning, and its kind no longer changes. Order is a position.
- **Settings are checked values.** Nothing points to a setting, so each is one value with a strict shape: no unknown or missing fields, no null where a value is required, and the key's own rules, such as rounding from 0 to 4 decimals, or a rain reference that is one of the rain values. A refusal names every problem.
- **Privacy rounding and the consent text are bounded, not hidden.** Both are on the admin page. A minor's positions are never rounded more finely than their consent's text stated, so loosening `privacy` reaches only consents given afterwards, whose guardians see the new figure. The consent text is published as a new version and never edited, so every consent keeps the words it was given against.
- **One way in.** Endpoints, commands and the seed write through the same functions and checks.
- **Logged.** Every change is logged in `configuration_changes`, with the admin's name at the time and a summary, newest first, as registry changes are.
- **At once.** Pages read the configuration on each request, so a change shows on the next one. The front end reads stages and regions through `publicSite.content`.

The admin endpoints, for platform admins only:

| Endpoint | Does |
|---|---|
| `GET /admin/stages`, `PUT /admin/stages/{stageId}`, `DELETE /admin/stages/{stageId}` | Lists, adds or changes, and deletes a stage not in use |
| `GET /admin/regions`, `PUT /admin/regions/{regionId}`, `DELETE /admin/regions/{regionId}` | The same for regions |
| `GET /admin/host-questions`, `PUT /admin/host-questions/{questionId}`, `POST /admin/host-questions/{questionId}/retire` | Lists, adds or changes, and retires a question |
| `GET /admin/settings`, `PUT /admin/settings/{key}` | Reads every setting, and replaces one |
| `GET /admin/consent-texts`, `GET /admin/consent-texts/{version}`, `POST /admin/consent-texts` | Lists the versions, reads one in every language, and publishes a new one |
| `PATCH /admin/classrooms/{classroomId}` | Sets a classroom's stage, region and profile fields; a field left out stays as it is |
| `GET /admin/configuration-changes` | The change log, a page at a time |

From a terminal:

```sh
dgs settings get privacy > privacy.json   # edit, then
dgs settings put privacy privacy.json
dgs classroom configure DGC-HG2W5K classroom.json
dgs consent publish consent.json            # a new version, every language
```

- A classroom file holds the same fields as `PATCH /admin/classrooms/{classroomId}`: any of `stageId`, `regionId` (null clears it) and `profile` fields. Profile fields replace their own value only.
- What belongs to other modules stays there: a lab's parameter notes are set with labs; templates are registry entries.
- **The seed.** `dgs seed` loads the overlay and the default alert levels, the stages, regions and host questions, the settings, and the first consent version while there is none, from the service's own seed, built into the binary. It adds a list's rows only while the list is empty, so a stage an admin deleted stays deleted. On a re-run it updates the rows and settings it wrote that no admin has changed since, and keeps those an admin has, so it is safe in every environment.

## Alternatives considered

- **Presentation in SAM.** Labels, charts and tiles are DaGrouper's, and a second tenant would not use them unchanged. Rejected for the overlay.
- **Admins holding `registry:manage` themselves.** Platform admin is this service's role, and SAM would see admins as registry authorities for every purpose. Rejected: this service sends the entries and records the admin in its change log.
- **Calling SAM for each admin change.** The page would wait on SAM and fail in an outage. Rejected: the definition is shown at once and sent through the outbox.
- **One JSON value per key for all configuration, edited by command.** Lists that other rows point to would have no foreign keys: a region in use could be removed, and a question's answers would lose their meaning. Rejected for lists; kept for settings, which nothing points to.
- **The site's words as configuration.** They would be kept in one language, apart from the front end's translations, and checked by a service that never reads them. Rejected: they live in the front end.
- **Privacy rounding and the consent text by command only.** Admins would need someone with access to the deployment for changes that are theirs to make. Rejected: both are on the admin page, bounded by the consent's promise and kept in versions.
- **The consent text in the front end, with the site's words.** A release would change what guardians agree to with no record, and no consent could name the text it was given against. Rejected: it is kept here, in versions.
- **Configuration in SAM's back office.** Stages, regions and questions mean nothing to another tenant. Rejected: SAM's back office runs the platform, and this service keeps its own configuration.
- **Configuration in environment variables or files read at start-up.** A change would need a restart, and nothing would check the shapes the pages read. Rejected.

## Risks

| Risk | Mitigation |
|---|---|
| SAM refuses a tenant entry | The row is held for an operator; the admin page shows the definition, so an admin may see a value devices cannot yet send |
| SAM's registry is unreachable | The cached copy is served |
| A configuration mistake breaks a page | Checked before it is stored |
| An admin removes a stage, region or question that is in use | Foreign keys refuse it; questions with answers are retired instead |
| A setting change alters every station page, or a new classroom's terms | Logged with the admin's name; the checks bound each value |
| An admin loosens `privacy` or changes the consent text | Minors already consented keep the rounding their text stated; new guardians see the new figure and words; each change is a logged version or setting with the admin's name |

## Testing approach

- **Unit:** the merge of SAM's registry with the overlay and own entries; unit conversion; each setting's checks.
- **Integration:** only this tenant's and core entries show; a new value reaches SAM as `dgs:<id>`; a core device type's values are refused; a device type waits for its parameters; a setting with a bad shape is refused; a stage or region in use cannot be deleted; a question with answers is retired, not deleted; loosening `privacy` never rounds a minor more finely than their consent stated; a published consent version cannot be changed; a seed re-run keeps an admin's changes.

## Open questions

- **Translated labels.** Stage, region and question labels are kept in the site's default language. When the front end translates the site, they need a label per language, or keys the front end translates.
