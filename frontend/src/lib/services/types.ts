import type {
	Address,
	ClassroomDetail,
	ClassroomRole,
	ClassroomSummary,
	CreateClassroomInput,
	Invitation,
	InvitationPreview,
	AuthResult,
	Parameter,
	PhoneChallenge,
	PlaceSuggestion,
	ProfileInput,
	Station,
	StationSnapshot,
	User
} from '#lib/domain/types.ts';

/**
 * The contract between the UI and its data source. Pages only talk to these interfaces
 * through `services` in `#lib/services`, so swapping the mock layer for the real auth API
 * and SAM means providing another implementation, not changing pages.
 *
 * All methods reject with a `ServiceError` on failure.
 */

export interface AuthService {
	/** The signed-in user, or null. Real backend: reads the HTTP-only session cookie. */
	getCurrentUser(): Promise<User | null>;
	/** Exchange a Google ID token for a session. The backend verifies the token. */
	signInWithGoogle(idToken: string): Promise<AuthResult>;
	/** Validate the number and send a 6-digit SMS code. */
	requestPhoneCode(phone: string): Promise<PhoneChallenge>;
	/** Verify the SMS code and start a session. */
	verifyPhoneCode(requestId: string, code: string): Promise<AuthResult>;
	signOut(): Promise<void>;
}

export interface ProfileService {
	updateProfile(input: ProfileInput): Promise<User>;
	/** Google sign-ups verify a phone number during onboarding. */
	startPhoneVerification(phone: string): Promise<PhoneChallenge>;
	confirmPhoneVerification(requestId: string, code: string): Promise<User>;
	updateAddress(address: Address): Promise<User>;
	/** Checks required fields and sets `onboardingCompleted`. */
	completeOnboarding(): Promise<User>;
}

export interface PlacesService {
	autocomplete(query: string): Promise<PlaceSuggestion[]>;
	getAddress(placeId: string): Promise<Address>;
}

export interface RegistryService {
	listParameters(): Promise<Parameter[]>;
	listStations(): Promise<Station[]>;
	getLatestSnapshot(stationId: string): Promise<StationSnapshot | null>;
}

/**
 * Classrooms (AP-03). Permission checks and code redaction happen behind this interface,
 * never only in the UI.
 */
export interface ClassroomService {
	listMine(): Promise<ClassroomSummary[]>;
	get(classroomId: string): Promise<ClassroomDetail>;
	/** Institution heads create their own; platform admins can set one up for a head. */
	create(input: CreateClassroomInput): Promise<ClassroomSummary>;
	/** Sends one invitation email per address. */
	invite(classroomId: string, emails: string[], role: ClassroomRole): Promise<Invitation[]>;
	revokeInvitation(invitationId: string): Promise<void>;
	setMemberRole(classroomId: string, userId: string, role: ClassroomRole): Promise<void>;
	removeMember(classroomId: string, userId: string): Promise<void>;
	/** Public: what an invite link points to. */
	getInvitation(token: string): Promise<InvitationPreview>;
	/** Joins the signed-in user to the classroom. Their email must match the invitation. */
	acceptInvitation(token: string): Promise<ClassroomSummary>;
}

export interface Services {
	auth: AuthService;
	profile: ProfileService;
	classrooms: ClassroomService;
	places: PlacesService;
	registry: RegistryService;
}

export type ServiceErrorCode =
	| 'unauthenticated'
	| 'invalid_input'
	| 'not_found'
	| 'rate_limited'
	| 'code_invalid'
	| 'code_expired'
	| 'conflict'
	| 'forbidden'
	| 'network';

export class ServiceError extends Error {
	constructor(
		readonly code: ServiceErrorCode,
		message: string,
		/** Field-level messages, keyed by input name. */
		readonly fields: Record<string, string> = {}
	) {
		super(message);
		this.name = 'ServiceError';
	}
}

export function errorMessage(e: unknown): string {
	if (e instanceof ServiceError) return e.message;
	return 'Something went wrong. Please try again.';
}
