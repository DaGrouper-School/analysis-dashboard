# DaGrouper Analysis Dashboard

Front end for the DaGrouper Analysis Platform, built with SvelteKit (Svelte 5) and Tailwind CSS.

For now it is front-end only: data comes from a mock layer that is prefilled with demo data and
saved in the browser's localStorage. The auth API and SAM (Scientific Asset Manager) backend will
replace it later without changes to the pages.

## Getting started

Install from the repo root (this folder is an npm workspace), then run the app:

```sh
npm install          # at the repo root
npm run dev          # at the root, or here
```

Other scripts, run here: `npm run build` (static SPA in `build/`), `npm test`, `npm run check`,
`npm run lint`.

## Demo accounts

| Sign in with | Account                              | State                                                   |
| ------------ | ------------------------------------ | ------------------------------------------------------- |
| Google       | Tesfaye G.                           | Institution head of Trinity Lab Classroom               |
| Google       | Dr. Marcus L.                        | Researcher in Trinity Lab Classroom, and platform admin |
| Google       | Sarah K.                             | Custodian, not in a classroom                           |
| Google       | Hana Bekele, Amina Yusuf             | New, goes through onboarding                            |
| Phone        | Ethiopia +251 911 000 001 (Abebe T.) | Community scientist in Trinity Lab Classroom            |
| Phone        | Any other valid number               | New                                                     |

Hana Bekele has a pending facilitator invitation at `/invite/demo-hana-invite`.

No SMS is sent: the mock shows the 6-digit code on screen. "Reset demo data" on the profile page
restores the seed.

## How the data layer is organised

```
src/lib/
  domain/        types and validation shared by the UI and every data source
  services/
    types.ts     the service interfaces (auth, profile, classrooms, places, registry): the UI's only contract
    index.ts     picks the implementation from VITE_DATA_SOURCE
    mock/        localStorage-backed implementation that behaves like the backend
    http/        fetch-based sketch of the real implementation (proposed endpoints)
  data/          mock database (localStorage persistence) and seed data
```

Pages never touch localStorage or fetch directly. They call `services.*` from
`#lib/services/index.ts`. To move to the real backend, finish `services/http`, set
`VITE_DATA_SOURCE=http` and `VITE_API_URL`, and the same pages run against it. The mock enforces
the same rules the backend should (OTP expiry, 30-second resend, attempt limits, unique
email/phone, required onboarding fields) so the UI handles those errors already.

Authentication: in the mock, the "session" is a record in localStorage. With the real backend it
becomes an HTTP-only cookie set by the server (AP-02), so the front end never handles tokens;
`services.auth.getCurrentUser()` is the only way pages learn who is signed in. Google sign-in in
the mock is an account picker that returns a fake ID token; the real flow uses Google Identity
Services and the backend verifies the token.

## Routes

| Route                 | Story                                                   |
| --------------------- | ------------------------------------------------------- |
| `/signup`             | AP-01 sign up with Google or phone                      |
| `/verify-phone`       | AP-01 SMS code with 30 s resend                         |
| `/onboarding/profile` | AP-02 profile and contact (Google users verify a phone) |
| `/onboarding/address` | AP-02 address with places autocomplete                  |
| `/dashboard`          | Current conditions from the user's own classrooms only  |
| `/map`                | AP-06 device map with current conditions and alerts     |
| `/classrooms/[id]`    | AP-03 classroom, members, invites; edit name and school |
| `/labs/[labId]`       | AP-05 devices, readings, connect-a-device demo          |
| `/labs/[labId]/edit`  | Edit the lab and its site profile                       |
| `/profile`            | Edit profile and address, sign out                      |

Route groups `(auth)` and `(app)` hold the guards: signed-in users skip sign-up, and the app
requires a completed onboarding.

## Seed data

Personas come from the project vault (User Personas). The parameter registry holds the 36 weather
station parameters mapped to the six environmental domains (AIR, WAVES, SUN, RAIN, WIND,
LIGHTNING) plus the manual rain gauge, with values from the sample station payload.

Classrooms run labs, and devices are registered to a lab. Registration names the device only:
GPS belongs to each reading, so a device can report from different places over time. Trinity Lab
(Addis Ababa) has a soil sensor in its garden plus a weather station and rain gauge in Shashamane,
about 200 km away. The Shashamane station's two readings come from two slightly different spots;
the rain gauge and the Hargeisa station send no GPS, so their readings use the lab's location and
are flagged as not from the device. Each device's SAM source ID is the classroom code plus the
device identifier, e.g. `DGC-7K3Q9M:TWS-2041-SH`, and every reading is tagged with its classroom.

Classrooms have four roles: institution head, facilitator, community scientist and researcher.
Researchers are read-only members invited by the head: they see the classroom's devices and
readings but cannot register devices, upload readings or see the classroom code.

People are never assigned to devices. Current conditions come only from devices in classrooms the
user belongs to; anyone else, platform admins included, sees an empty state. On a lab page,
facilitators and the institution head can run the connect-a-device demo: it pairs with a
registered device and records six readings, two of them without a GPS fix, which then appear under Recent readings on the dashboard. Classroom name and
institution are editable by the head; the classroom code never changes.

## Look and feel

There are dark and light themes. The app starts in the system's choice; the sun and moon button in
the header switches it, and the choice is saved in localStorage. The dark theme uses a black
canvas, thin white lines, white pills for the active choice, Barlow type, and green, teal and cyan
for data. Tokens live in `src/routes/layout.css`, and the light theme overrides them under
`[data-theme='light']`. The map is a dotted globe drawn on a canvas (`Globe.svelte`); its land
dots come from `src/lib/data/land-dots.json`, built from Natural Earth by `node
scripts/generate-land-dots.mjs`.
