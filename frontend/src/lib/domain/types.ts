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

export type ClassroomRole = 'institution_head' | 'facilitator' | 'community_scientist';

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

export interface CreateClassroomInput {
	name: string;
	institutionName: string;
	/** Admin-assisted setup: invite this person as institution head instead of joining yourself. */
	headEmail?: string;
}

// ---- Parameter registry (SAM-01, SAM-05) -------------------------------------------

export type EnvironmentalDomain = 'AIR' | 'WAVES' | 'SUN' | 'RAIN' | 'WIND' | 'LIGHTNING';

export interface Parameter {
	/** Stable registry ID, e.g. `air_temperature`. */
	id: string;
	label: string;
	definition: string;
	unit: string;
	domain: EnvironmentalDomain;
	source: 'weather_station' | 'manual';
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

export type DeviceType =
	'weather_station' | 'soil_sensor' | 'rain_gauge' | 'water_probe' | 'air_quality_sensor' | 'other';

/** A sensor registered to a lab. It may have its own GPS position, which can be far from the lab. */
export interface Device {
	id: string;
	labId: string;
	labName: string;
	name: string;
	type: DeviceType;
	/** Manufacturer serial or other hardware identifier. */
	identifier: string;
	/** Where the device reports from: its own GPS, or the lab's location when it has none. */
	location: GeoPoint;
	/** False when the device has no GPS of its own and uses the lab's location. */
	ownLocation: boolean;
	/** Human-readable place of the device, e.g. "Shashamane". */
	placeName: string;
	country: string;
	/**
	 * SAM source identity: classroom code plus device identifier (SAM-06).
	 * Null when the viewer may not see the classroom code.
	 */
	samSourceId: string | null;
	registeredAt: string;
}

export interface LabDetail {
	lab: Lab;
	classroomName: string;
	devices: Device[];
	/** Heads and facilitators can register devices. */
	canManage: boolean;
}

export interface CreateLabInput {
	name: string;
	placeName: string;
	country: string;
	location: GeoPoint;
	siteProfile: SiteProfile;
}

export interface RegisterDeviceInput {
	name: string;
	type: DeviceType;
	identifier: string;
	placeName: string;
	country: string;
	/** Null: the device uses the lab's location. */
	location: GeoPoint | null;
}

/** Latest value per parameter for a device, keyed by parameter ID. */
export interface DeviceSnapshot {
	deviceId: string;
	observedAt: string;
	values: Record<string, number | string>;
}
