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
	createdAt: string;
	updatedAt: string;
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
		/** Heads and facilitators of the classroom set its alert levels (AP-08). */
		manageAlerts: boolean;
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
	/** Admin-assisted setup: invite this person as institution head instead of joining yourself. */
	headEmail?: string;
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
	/** Numbers chart as trends; text values (e.g. a trend word) only show as they are. */
	valueType?: ValueType;
	/** Plausible range in `unit`, checked before a manual observation is sent. */
	min?: number;
	max?: number;
	/** Other units people may measure in, converted to `unit` before sending. */
	altUnits?: AltUnit[];
	/** For text values: the answers to pick from. */
	options?: string[];
}

export type ValueType = 'number' | 'text';

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
 * A rule in force for a classroom. A classroom's own rules for a parameter replace the platform
 * defaults for that parameter; parameters it has not set keep the defaults.
 */
export interface EffectiveAlertRule extends AlertRule {
	source: 'classroom' | 'default';
	/** Who last changed a classroom's own rule, and when. */
	updatedBy: string | null;
	updatedAt: string | null;
}

export interface ClassroomAlerts {
	rules: EffectiveAlertRule[];
	canEdit: boolean;
}

/** The latest reading of one device in a classroom the user belongs to. */
export interface CurrentConditions {
	device: Device;
	classroomName: string;
	reading: Reading;
	/** Active alerts from the latest reading. Empty for older readings. */
	alerts: DeviceAlert[];
}
