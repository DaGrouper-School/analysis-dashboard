/**
 * Domain model shared by the UI and every data source (mock today, SAM / auth API later).
 * Keep these shapes backend-agnostic: no localStorage or fetch details belong here.
 */

export type AuthProvider = 'google' | 'phone';

export type UserRole =
	'community_scientist' | 'custodian' | 'facilitator' | 'institution_head' | 'researcher';

export interface Address {
	street: string;
	suite?: string;
	city: string;
	state: string;
	zip: string;
	country: string;
	/** Set when the address came from the places autocomplete. */
	placeId?: string;
	lat?: number;
	lng?: number;
}

export interface User {
	id: string;
	authProvider: AuthProvider;
	fullName: string;
	email: string | null;
	phone: string | null;
	phoneVerified: boolean;
	avatarUrl: string | null;
	role: UserRole;
	/** Platform administrators can set up classrooms for institutions (admin-assisted setup). */
	platformAdmin: boolean;
	address: Address | null;
	onboardingCompleted: boolean;
	/** Under 18 (AP-04): contributes only while a guardian's consent is recorded. */
	minor: boolean;
	consent: ConsentStatus;
	createdAt: string;
	updatedAt: string;
}

/**
 * A guardian's consent for a minor (AP-04). `not_needed` for adults; a minor starts at
 * `pending` and can send observations only while it is `given`.
 */
export type ConsentStatus = 'not_needed' | 'pending' | 'given' | 'withdrawn';

/** Who to ask for consent: a name, and an email address or phone number. */
export interface GuardianInput {
	guardianName: string;
	guardianContact: string;
}

/**
 * How a guardian's decision reached us: their link (AP-04), or a signed paper sheet that the
 * classroom's head or a facilitator recorded (AP-38).
 */
export type ConsentMethod = 'link' | 'paper';

/** The signed-in minor's consent, as they see it. */
export interface GuardianConsent {
	status: ConsentStatus;
	method: ConsentMethod;
	guardianName: string;
	/** Empty for a paper sheet: nobody was sent a link. */
	guardianContact: string;
	requestedAt: string;
	decidedAt: string | null;
	/** The day the guardian signed the paper sheet (YYYY-MM-DD); null for a link. */
	signedOn: string | null;
	/** Who recorded the paper sheet; null for a link. */
	recordedByName: string | null;
	/** Mock only: the link the guardian would receive. The real backend never returns it. */
	devToken: string | null;
}

/** What the guardian sees when they open their link. No sign-in. */
export interface ConsentRequest {
	/** The contributor's first name only. */
	contributorName: string;
	guardianName: string;
	classroomNames: string[];
	status: ConsentStatus;
	decidedAt: string | null;
	/** Positions from the contributor's phone are rounded to this many decimal places. */
	coordinateDecimals: number;
}

export interface AuthResult {
	user: User;
	/** True when this sign-in created the account. */
	isNewUser: boolean;
}

/** An SMS one-time-code challenge (AP-01 phone sign-up, AP-02 phone verification). */
export interface PhoneChallenge {
	requestId: string;
	phone: string;
	/** ISO time after which a new code may be requested (30 s resend timer). */
	resendAvailableAt: string;
	expiresAt: string;
	/** Mock data source only: the code a real SMS gateway would have sent. */
	devCode?: string;
}

export interface ProfileInput {
	fullName: string;
	email: string;
}

export interface PlaceSuggestion {
	placeId: string;
	primaryText: string;
	secondaryText: string;
}

// ---- Classrooms (AP-03) -------------------------------------------------------------

export type ClassroomRole =
	| 'institution_head'
	| 'facilitator'
	| 'community_scientist'
	/** Read-only: sees the classroom's devices and readings, invited by the head. */
	| 'researcher';

export interface Classroom {
	id: string;
	name: string;
	institutionName: string;
	/**
	 * Unique global classroom code, also the classroom's source identifier in SAM.
	 * Null when the viewer is contributor tier: the code is never sent to them.
	 */
	code: string | null;
	setup: 'self_service' | 'admin_assisted';
	createdAt: string;
}

export interface ClassroomSummary extends Classroom {
	/** The viewer's role; null for a platform admin who is not a member. */
	myRole: ClassroomRole | null;
	memberCount: number;
}

export interface ClassroomMember {
	userId: string;
	fullName: string;
	email: string | null;
	role: ClassroomRole;
	joinedAt: string;
}

export type InvitationStatus = 'pending' | 'accepted' | 'revoked';

export interface Invitation {
	id: string;
	classroomId: string;
	email: string;
	role: ClassroomRole;
	status: InvitationStatus;
	invitedByName: string;
	createdAt: string;
	/** Secret part of the invite link. The backend emails the link; the mock shows it. */
	token: string;
}

/** A minor in a classroom and their guardian's consent (AP-38), for its head and facilitators. */
export interface MemberConsent {
	userId: string;
	fullName: string;
	status: ConsentStatus;
	/** Null while nothing was asked or recorded. */
	method: ConsentMethod | null;
	guardianName: string | null;
	signedOn: string | null;
	/** When the decision was recorded or answered. */
	decidedAt: string | null;
	recordedByName: string | null;
}

/** A paper consent sheet, as the head or a facilitator records it (AP-38). */
export interface PaperConsentInput {
	/** True for consent given, false for a withdrawal. */
	give: boolean;
	guardianName: string;
	/** The day on the sheet, YYYY-MM-DD. Not in the future. */
	signedOn: string;
}

/** The printable consent sheet's words (AP-38), edited as content rather than code. */
export interface ConsentSheetContent {
	title: string;
	/**
	 * Paragraphs and sections may use `{classroom}`, `{institution}` and `{decimals}`, filled in
	 * for each classroom.
	 */
	intro: string;
	sections: { heading: string; body: string }[];
	/** The two boxes the guardian ticks. */
	choices: { give: string; withdraw: string };
	/** The line for the young contributor's name, filled in when the sheet is for one minor. */
	contributorField: string;
	/** Lines left blank for the guardian to fill in by hand. */
	fields: string[];
	/** For the classroom only, below the signature: how to record the sheet. */
	officeNote: string;
}

/** A classroom's printable consent sheet, with its words filled in (AP-38). */
export interface ConsentSheet {
	classroomName: string;
	institutionName: string;
	content: ConsentSheetContent;
	/** The classroom's minors, to print a sheet with a name already on it. */
	minors: { userId: string; fullName: string }[];
}

export interface ClassroomDetail {
	classroom: ClassroomSummary;
	members: ClassroomMember[];
	/** Only for people who can manage the classroom; empty otherwise. */
	invitations: Invitation[];
	permissions: {
		invite: ClassroomRole[];
		manageMembers: boolean;
		/** Can change the name and institution. The code never changes. */
		edit: boolean;
		/** Can see the classroom code: platform admins, the institution head and facilitators. */
		viewCode: boolean;
		/** Can see and record minors' guardian consent: the institution head and facilitators. */
		manageConsent: boolean;
	};
}

/** What the invite page shows before the user signs in. */
export interface InvitationPreview {
	token: string;
	classroomName: string;
	institutionName: string;
	role: ClassroomRole;
	email: string;
	status: InvitationStatus;
	invitedByName: string;
}

/** Everything about a classroom except its code, which is its permanent identifier. */
export interface UpdateClassroomInput {
	name: string;
	institutionName: string;
}

export interface CreateClassroomInput {
	name: string;
	institutionName: string;
	/** Only platform admins create classrooms. Creating one invites this person as its head. */
	headEmail: string;
	/** The accepted host application (AP-30) it is set up from, if any. */
	applicationId?: string;
}

// ---- Parameter registry (SAM-01, SAM-05) -------------------------------------------

/** An environmental domain's short code, e.g. `AIR` or `SOIL`. Platform admins add domains. */
export type EnvironmentalDomain = string;

/** An environmental domain in the registry (SAM-23). Shared by every classroom. */
export interface DomainDefinition {
	/** Short code, upper case, e.g. `SOIL`. Never changes once created. */
	id: EnvironmentalDomain;
	label: string;
	/** A registry icon name (see `#lib/icons.ts`), shown on tabs and cards. */
	icon: string;
	description: string;
}

export interface Parameter {
	/** Stable registry ID, e.g. `air_temperature`. */
	id: string;
	label: string;
	definition: string;
	unit: string;
	/** The domain this value shows under. Null until a platform admin maps it. */
	domain: EnvironmentalDomain | null;
	source: 'weather_station' | 'manual' | 'device';
	/** Numbers chart as trends; text values (e.g. a trend word) show as they are, times as dates. */
	valueType?: ValueType;
	/** Plausible range in `unit`, checked before a manual observation is sent. */
	min?: number;
	max?: number;
	/** Other units people may measure in, converted to `unit` before sending. */
	altUnits?: AltUnit[];
	/** For text values: the answers to pick from. */
	options?: string[];
	/**
	 * What the value means, in one plain sentence for someone without a science background
	 * (AP-07). States what is measured; never advice, never a cause.
	 */
	plain?: string;
	/** Changes smaller than this, in `unit`, read as steady: the sensor's own wobble. */
	noise?: number;
	/** How SAM gets the value: measured, derived from other values, or entered by hand. */
	calculation?: Calculation;
}

export interface Calculation {
	method: 'measured' | 'derived' | 'manual';
	/** The formula for a derived value, or how a measured one is taken. */
	formula: string;
	/** Registry IDs of the values the formula uses. */
	inputs: string[];
	/** The SAM registry version that defines it, and when that version was published. */
	version: string;
	publishedAt: string;
}

/** What a platform admin sets for a value's "What it means" and "How is this calculated?". */
export interface ExplanationInput {
	plain: string;
	method: Calculation['method'];
	formula: string;
	inputs: string[];
}

/** What "How is this calculated?" shows: the calculation with its inputs named. */
export interface CalculationDetails extends Calculation {
	parameter: Pick<Parameter, 'id' | 'label' | 'unit' | 'source' | 'plain'>;
	inputDetails: Pick<Parameter, 'id' | 'label' | 'unit'>[];
}

/** `time` is a moment as Unix seconds: shown as a local date, never charted. */
export type ValueType = 'number' | 'text' | 'time';

/** Another unit for a value: `value * factor + offset` gives it in the parameter's own unit. */
export interface AltUnit {
	unit: string;
	factor: number;
	offset?: number;
}

/** One field of an observation template: a registry value, and whether it must be filled in. */
export interface TemplateField {
	parameterId: string;
	required: boolean;
}

/**
 * What a manual observation asks for, per domain (AP-16): a rain gauge reading, a coral spawning
 * check, a species sighting. A template with a device type is read off a registered device of
 * that type; one without is a field observation with no device.
 */
export interface ObservationTemplate {
	id: string;
	label: string;
	description: string;
	domain: EnvironmentalDomain | null;
	deviceType: DeviceType | null;
	fields: TemplateField[];
}

/** A kind of device and the values it is expected to send (SAM-23). Shared by every classroom. */
export interface DeviceTypeDefinition {
	/** Stable ID, e.g. `soil_sensor`. */
	id: DeviceType;
	label: string;
	description: string;
	/** Marker and chip color. */
	color: string;
	/** A registry icon name (see `#lib/icons.ts`), picked by a platform admin. */
	icon: string;
	/** Registry parameters the device sends, in display order. */
	parameterIds: string[];
	/**
	 * Entered by hand, with no sensor: added to a lab without pairing, with a generated
	 * identifier, and recorded with "Enter measurement" instead of a sync.
	 */
	manual?: boolean;
}

/** One change to the registry, kept so stewards can see who changed what. */
export interface RegistryChange {
	id: string;
	at: string;
	byName: string;
	summary: string;
}

export interface DomainInput {
	id: string;
	label: string;
	icon: string;
	description: string;
}

/** A value a new device type sends. An ID already in the registry reuses that parameter. */
export interface ExpectedValueInput {
	id: string;
	label: string;
	unit: string;
	valueType: ValueType;
	/** Used when the value is new to the registry; existing values keep their mapping. */
	domain: EnvironmentalDomain | null;
}

export interface DeviceTypeInput {
	label: string;
	description: string;
	color: string;
	/** A registry icon name. Blank keeps the current icon, or the generic sensor for a new type. */
	icon?: string;
	/** Entered by hand, with no sensor attached. */
	manual?: boolean;
	values: ExpectedValueInput[];
}

// ---- Labs and devices (AP-05) --------------------------------------------------------

export interface GeoPoint {
	lat: number;
	lng: number;
}

export type SiteSetting = 'coastal' | 'urban' | 'rural' | 'highland' | 'forest' | 'other';

/** Onboarding questionnaire about the lab's site. */
export interface SiteProfile {
	setting: SiteSetting;
	description: string;
	researchQuestions: string[];
}

/** A Lab in a Bag, run by a classroom. Devices are registered to it. */
export interface Lab {
	id: string;
	classroomId: string;
	name: string;
	/** Where the lab itself is, e.g. the school. */
	placeName: string;
	country: string;
	location: GeoPoint;
	siteProfile: SiteProfile;
	createdAt: string;
}

/** A device type's ID. Built in: weather_station, soil_sensor, rain_gauge, water_probe,
 * air_quality_sensor, other. Platform admins register more. */
export type DeviceType = string;

/**
 * A sensor registered to a lab. A device has no fixed position: GPS travels with each reading,
 * so the same device can report from different places over time. People are never assigned
 * to devices; access comes from classroom membership.
 */
export interface Device {
	id: string;
	labId: string;
	labName: string;
	name: string;
	type: DeviceType;
	/** Manufacturer serial or other hardware identifier. */
	identifier: string;
	/**
	 * SAM source identity: classroom code plus device identifier (SAM-06).
	 * Null when the viewer may not see the classroom code.
	 */
	samSourceId: string | null;
	registeredAt: string;
}

/**
 * Where a reading's position came from. `device`: GPS sent with the measurement.
 * `lab`: the device sent no GPS, so the lab's location stands in. It is not the device's position.
 */
export type ReadingLocationSource = 'device' | 'lab';

/** One measurement upload from a device, as stored by the backend and forwarded to SAM. */
export interface Reading {
	id: string;
	deviceId: string;
	labId: string;
	/** Every reading is tagged with the classroom that owns the device. */
	classroomId: string;
	observedAt: string;
	receivedAt: string;
	/** Values keyed by parameter ID. */
	values: Record<string, number | string>;
	location: GeoPoint;
	locationSource: ReadingLocationSource;
	/**
	 * The person who recorded it, shown only if they chose to take credit. Otherwise null and the
	 * reading is attributed to the classroom.
	 */
	creditedTo: string | null;
}

/** Options for recording readings. */
export interface RecordOptions {
	/** Show the recorder's name on these readings. Off unless they opt in. */
	credit: boolean;
}

/** What a device sends when it connects. */
export interface ReadingInput {
	observedAt: string;
	values: Record<string, number | string>;
	/** Null when the device has no GPS fix or no GPS at all. */
	location: GeoPoint | null;
}

/** A manual observation, as sent from the guided form. Values are in each parameter's own unit. */
export interface ObservationInput {
	templateId: string;
	labId: string;
	/** The registered device it was read from. Null for a field observation. */
	deviceId: string | null;
	observedAt: string;
	values: Record<string, number | string>;
	/** From the phone's GPS. Null when there is no fix: the lab's location stands in. */
	location: GeoPoint | null;
}

/**
 * A stored manual observation. One read off a device is also one of that device's readings;
 * a field observation is sent to SAM under the classroom.
 */
export interface Observation extends Omit<Reading, 'deviceId'> {
	templateId: string;
	deviceId: string | null;
}

/**
 * Something wrong with the device's clock, found when a batch is synced: the clock was more than
 * a few minutes off the server's, or the observation is dated after the device sent it.
 */
export type ClockFlag = 'device_clock_off' | 'observed_after_sent';

/** One observation saved on the device, sent in a sync batch. */
export interface SyncRecord {
	/** Made on the device when the observation was saved; a retry sends the same one. */
	clientId: string;
	observation: ObservationInput;
	credit: boolean;
}

/** Observations saved offline, sent together once the device is back online. */
export interface SyncBatch {
	/** The device clock when the batch was sent, so the server can check it. */
	sentAt: string;
	records: SyncRecord[];
}

/**
 * `stored`: new. `duplicate`: already stored from an earlier attempt, returned as it was.
 * `rejected`: not stored; `error` says why.
 */
export type SyncOutcome = 'stored' | 'duplicate' | 'rejected';

export interface SyncRecordResult {
	clientId: string;
	outcome: SyncOutcome;
	observation: Observation | null;
	error: string | null;
	/** Field-level messages when rejected, keyed by input name. */
	fields: Record<string, string>;
	clockFlags: ClockFlag[];
}

export interface SyncBatchResult {
	receivedAt: string;
	/** Server clock minus the device clock when the batch arrived, in ms. */
	clockOffsetMs: number;
	/** One per record, in the batch's order. */
	results: SyncRecordResult[];
}

/** A lab the user may record in, with its registered devices. */
export interface ObservationTarget {
	lab: Lab;
	classroomName: string;
	devices: Device[];
}

export interface LabDetail {
	lab: Lab;
	classroomName: string;
	devices: Device[];
	/** Recent readings from this lab's devices, newest first. */
	readings: Reading[];
	/** Field observations (no device) recorded in this lab, newest first. */
	observations: Observation[];
	/** Heads and facilitators can register devices and edit the lab. */
	canManage: boolean;
	/** Can connect a registered device and record its readings (also community scientists). */
	canRecord: boolean;
}

export interface CreateLabInput {
	name: string;
	placeName: string;
	country: string;
	location: GeoPoint;
	siteProfile: SiteProfile;
}

/** Registration identifies the device only. Location arrives later, with its readings. */
export interface RegisterDeviceInput {
	name: string;
	type: DeviceType;
	identifier: string;
}

/** A condition in a device's latest reading that crosses a threshold. */
export interface DeviceAlert {
	parameterId: string;
	level: 'caution' | 'warning';
	title: string;
	message: string;
}

// ---- Alert levels (AP-08) ------------------------------------------------------------

export type AlertLevel = DeviceAlert['level'];
export type AlertDirection = 'above' | 'below';

/**
 * One alert level on one parameter: a value above (or below) `value`, in the parameter's own
 * unit, raises an alert. The same levels are the thresholds the dashboard shows.
 */
export interface AlertRule {
	id: string;
	parameterId: string;
	direction: AlertDirection;
	value: number;
	level: AlertLevel;
	/** Short name on the alert, e.g. `Very high UV`. States the reading; gives no advice. */
	title: string;
	/** Off keeps the rule without raising it. */
	enabled: boolean;
}

export type AlertRuleInput = Omit<AlertRule, 'id'>;

/**
 * Where a rule applies: one device, or every device of a sensor type in a classroom. For each
 * parameter, a device's own rules win over its type's, and those over the platform defaults.
 */
export type AlertScope = 'device' | 'type';

/** A rule in force for one device, with the layer it comes from. */
export interface EffectiveAlertRule extends AlertRule {
	source: AlertScope | 'default';
	/** Who last changed a device or type rule, and when. */
	updatedBy: string | null;
	updatedAt: string | null;
}

export interface DeviceAlerts {
	deviceId: string;
	deviceName: string;
	deviceType: DeviceType;
	classroomName: string;
	rules: EffectiveAlertRule[];
	/** Heads and facilitators set one device's levels. */
	canEditDevice: boolean;
	/** Only the institution head sets the classroom's levels for a sensor type. */
	canEditType: boolean;
}

/** The latest reading of one device in a classroom the user belongs to. */
export interface CurrentConditions {
	device: Device;
	classroomName: string;
	reading: Reading;
	/** Active alerts from the latest reading. Empty for older readings. */
	alerts: DeviceAlert[];
}

/**
 * The terms attached to a classroom's data in SAM (AP-14). Every export of it carries them, so
 * the data keeps its licence, credit and restrictions once it leaves the dashboard.
 */
export interface DataTerms {
	/** Licence name, e.g. "CC BY 4.0". */
	licence: string;
	licenceUrl: string | null;
	/** How to credit the data in a paper, poster or report. */
	attribution: string;
	/** Limits on use or sharing; empty when there are none. */
	restrictions: string[];
	/** Positions are rounded to this many decimal places before they leave SAM; null for full. */
	coordinateDecimals: number | null;
}

/** Columns the raw data table sorts by. */
export type RawSortKey = 'time' | 'parameter' | 'source' | 'site';

/** What to browse or export from one classroom. Unset filters include everything. */
export interface RawQuery {
	classroomId: string;
	/** First day included, `YYYY-MM-DD` in UTC. */
	from?: string;
	/** Last day included, `YYYY-MM-DD` in UTC. */
	to?: string;
	parameterId?: string;
	/** A device ID, or `field` for field observations. */
	sourceId?: string;
	labId?: string;
	sort?: RawSortKey;
	order?: 'asc' | 'desc';
}

/** One value of one reading or field observation, as SAM's export service returns it. */
export interface RawRow {
	/** The reading's ID plus the parameter ID: unique per row. */
	id: string;
	observedAt: string;
	receivedAt: string;
	parameterId: string;
	value: number | string;
	sourceId: string;
	sourceName: string;
	sourceKind: 'device' | 'field';
	labId: string;
	siteName: string;
	location: GeoPoint;
	locationSource: ReadingLocationSource;
	creditedTo: string | null;
}

/** A page of raw values from one classroom, with the filters it can offer and its terms. */
export interface RawPage {
	classroomId: string;
	classroomName: string;
	rows: RawRow[];
	/** Rows matching the filters, before the page limit. */
	total: number;
	terms: DataTerms;
	options: {
		parameterIds: string[];
		sources: { id: string; name: string; kind: RawRow['sourceKind'] }[];
		sites: { id: string; name: string }[];
	};
}

/** A CSV file from SAM's export service, ready to save. */
export interface CsvExport {
	filename: string;
	content: string;
	rows: number;
}

/** A top-level section of the public site (AP-20). `href` is null until it is built. */
export interface SiteSection {
	id: string;
	label: string;
	description: string;
	href: string | null;
}

/** A Global Classroom's stage on the way to operating, e.g. pilot. */
export interface ClassroomStage {
	id: string;
	label: string;
	description: string;
	/** Marker and badge color, as a CSS color. */
	color: string;
}

/** The public site's words and structure, edited as content rather than code. */
export interface SiteContent {
	hero: { eyebrow: string; title: string; body: string };
	/** Footer line naming the programme. */
	programme: string;
	sections: SiteSection[];
	/** Footer links: data terms, privacy and consent, safeguarding. */
	policies: SiteSection[];
	stages: ClassroomStage[];
	/** The regions classrooms are grouped by in the directory (AP-28), in display order. */
	regions: { id: string; label: string }[];
	languages: { code: string; label: string; ready: boolean }[];
	/** True while the data shown is sample data: every page that shows it says so. */
	sampleData: boolean;
	/** The "Host a Global Classroom" application (AP-30): its words. */
	hosting: { title: string; intro: string; submitted: string };
	/** The "Join this classroom" box on a classroom profile (AP-29). */
	joinClassroom: { title: string; body: string; action: string };
	/** The printable guardian consent sheet (AP-38). */
	consentSheet: ConsentSheetContent;
}

/** Where a classroom's released data stands. */
export type ReviewState = 'released' | 'in_review' | 'not_released';

/**
 * The shared core readings of one classroom (AP-20), from released data only. Null means no
 * released data for it: shown as "no data", never as zero.
 */
export interface CoreReadings {
	/** The day the rain values count, `YYYY-MM-DD` in UTC: the latest released reading's day. */
	day: string | null;
	rainToday: number | null;
	rainLast7Days: number | null;
	/** Days of the seven with a rain total; under 7 means the total is partial. */
	rainDays: number;
	airTemperature: number | null;
	/** When the latest released reading reached the platform. */
	lastSync: string | null;
	review: ReviewState;
}

/** One core reading's detail on a station page (AP-22). */
export interface StationCoreItem {
	review: ReviewState;
	/** Sources with a value on the core day. */
	sources: number;
	/** How far apart those sources are, in the reading's unit; null with fewer than two. */
	spread: number | null;
}

/** The station page's detail on the core readings: review state, sources and today's range. */
export interface StationCore {
	rain: StationCoreItem;
	temperature: StationCoreItem & { low: number | null; high: number | null };
}

/** A classroom as the public sees it: no code, no members, positions rounded. */
export interface PublicClassroom {
	id: string;
	name: string;
	/** The host institution: null until a signed host agreement exists. */
	institutionName: string | null;
	/** Where its first lab is, rounded; null before it has a lab. */
	placeName: string | null;
	country: string | null;
	location: GeoPoint | null;
	stageId: string;
	/** Its region in `SiteContent.regions`; null until one is set. */
	regionId: string | null;
	/** What it studies: its labs' research questions. */
	themes: string[];
	core: CoreReadings;
}

/** Where a public value came from: a device by its name, or the classroom's field observations. */
export interface PublicSource {
	id: string;
	/** The device's name; null for field observations. */
	name: string | null;
	kind: 'device' | 'field';
}

/** The latest released value of one parameter from one source. */
export interface PublicValue {
	parameterId: string;
	value: number | string;
	observedAt: string;
	source: PublicSource;
	/**
	 * The last reading of each of the seven days up to this one, oldest first, for a sparkline.
	 * Null for a day with no reading; empty for values that are not numbers.
	 */
	last7Days: (number | null)[];
}

/** One domain's card on a station overview; `domain` null for values with no domain yet. */
export interface PublicDomainCard {
	domain: Pick<DomainDefinition, 'id' | 'label' | 'icon' | 'description'> | null;
	values: PublicValue[];
}

/** How much of the month one domain's protocol has received (AP-22). */
export interface DomainCompleteness {
	domain: Pick<DomainDefinition, 'id' | 'label' | 'icon'> | null;
	/** Expected slots (hours, days or weeks) that have a reading. */
	received: number;
	expected: number;
	/** Each day of the month so far, for values read daily or faster. Gaps stay gaps. */
	days: { day: string; status: 'complete' | 'partial' | 'missing' }[];
}

/** A classroom's station overview for visitors (AP-21): released data only. */
export interface PublicStation {
	content: SiteContent;
	classroom: PublicClassroom;
	units: PublicHome['units'];
	/** Registry entries for every parameter shown, by ID. */
	parameters: Parameter[];
	domains: PublicDomainCard[];
	/** Review state, sources and today's range behind the core readings (AP-22). */
	core: StationCore;
	/**
	 * "How complete is this month?": the month of the latest released reading, against the
	 * classroom's protocol. Null with no released data or no protocol.
	 */
	completeness: { month: string; domains: DomainCompleteness[] } | null;
	/** The classroom's site codes. Members only; empty for everyone else. */
	siteCodes: string[];
	/** The viewer belongs to this classroom, so their dashboard shows it with raw values too. */
	member: boolean;
}

/** One parameter of one classroom for visitors: its meaning and released values over time. */
export interface PublicParameterPage {
	content: SiteContent;
	classroom: PublicClassroom;
	parameter: Parameter;
	domain: Pick<DomainDefinition, 'id' | 'label' | 'icon'> | null;
	/** One series per source, oldest first. */
	series: { source: PublicSource; points: { observedAt: string; value: number | string }[] }[];
	member: boolean;
}

/** A site where a research project measures (from LP-14 proposals). */
export interface ProjectSite {
	code: string;
	name: string;
	ecosystem: string;
	/** Registry parameters measured there. */
	parameterIds: string[];
	/** How often, e.g. "Daily". */
	frequency: string;
	/** The same as a rate the app can count: 24 is hourly, 1 daily, 1/7 weekly. */
	readingsPerDay: number;
	yearsPlanned: number;
}

export interface ResearchProject {
	id: string;
	title: string;
	question: string;
	sites: ProjectSite[];
}

/** A session on the classroom's calendar (LP-06). */
export interface ClassroomSession {
	id: string;
	title: string;
	startsAt: string;
	place: string;
}

/** What a classroom says about itself on its public profile (AP-29). */
export interface ClassroomProfile {
	/** Country or area code, e.g. `ET-AA`. */
	areaCode: string;
	language: string;
	/** Who the learners are and which ecosystems they study. */
	description: string;
	ecosystems: string[];
	hostAgreementSigned: boolean;
	projects: ResearchProject[];
	sessions: ClassroomSession[];
	/** Who looks after the Lab in a Bag. Shown to members only. */
	custodianRole: ClassroomRole;
}

/** One kind of device in a classroom's Lab in a Bag, from its registered devices. */
export interface KitItem {
	deviceType: Pick<DeviceTypeDefinition, 'id' | 'label' | 'icon'>;
	count: number;
	/** What it measures: its domains by label, then any values with no domain yet. */
	measures: string[];
	/** Serials of the devices. Members only: empty for everyone else. */
	serials: string[];
}

/** A classroom's public profile (AP-29). */
export interface PublicClassroomProfile {
	content: SiteContent;
	classroom: PublicClassroom;
	profile: Pick<ClassroomProfile, 'areaCode' | 'language' | 'description' | 'ecosystems'>;
	/** Projects with each site's readings as registry labels. */
	projects: (Omit<ResearchProject, 'sites'> & {
		sites: (Omit<ProjectSite, 'parameterIds'> & { readings: string[] })[];
	})[];
	kit: KitItem[];
	/** Members only; null for everyone else. */
	custodianRole: ClassroomRole | null;
	/** Upcoming sessions, soonest first. */
	sessions: ClassroomSession[];
	/** People who opted in to credit, by name; everyone else only counted. Minors are never named. */
	contributors: { named: string[]; anonymous: number };
	member: boolean;
	/** The viewer can manage the classroom: show "Manage classroom". */
	canManage: boolean;
}

export interface PublicHome {
	content: SiteContent;
	classrooms: PublicClassroom[];
	/** Units of the core readings, from the registry. */
	units: { rain: string; temperature: string };
}

// ---- Hosting a Global Classroom (AP-30) ---------------------------------------------

/** A readiness question, from configuration. */
export interface HostQuestion {
	id: string;
	label: string;
	/** Shown under the question. */
	help: string;
	kind: 'yes_no' | 'number' | 'text';
	required: boolean;
}

export interface HostApplicationInput {
	institutionName: string;
	contactName: string;
	contactRole: string;
	contactEmail: string;
	/** Optional. */
	contactPhone: string;
	/** Answers by question ID: `yes`/`no`, a number or text, as entered. */
	answers: Record<string, string>;
	site: {
		placeName: string;
		country: string;
		ecosystems: string[];
		/** Who the learners would be. */
		learners: string;
		/** What the classroom wants to study. */
		studyGoals: string;
	};
}

export type HostApplicationStatus = 'submitted' | 'accepted' | 'declined';

export interface HostApplication extends HostApplicationInput {
	id: string;
	status: HostApplicationStatus;
	submittedAt: string;
	decidedAt: string | null;
	/** The classroom created from it, once there is one. */
	classroomId: string | null;
}

/** What the applicant gets back: the application and the secret part of its status link. */
export interface HostApplicationReceipt {
	application: HostApplication;
	/** The backend emails the status link; the mock returns it. */
	token: string;
}
