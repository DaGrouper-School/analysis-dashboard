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

export interface Station {
	id: string;
	name: string;
	type: 'weather_station' | 'rain_gauge';
	siteName: string;
	country: string;
	lat: number;
	lng: number;
}

/** Latest value per parameter for a station, keyed by parameter ID. */
export interface StationSnapshot {
	stationId: string;
	observedAt: string;
	values: Record<string, number | string>;
}
