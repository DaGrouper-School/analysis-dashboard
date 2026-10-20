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
