import type {
	HostApplication,
	HostQuestion,
	ClockFlag,
	ConsentMethod,
	NowTileConfig,
	ConsentStatus,
	AlertRule,
	DataTerms,
	SiteContent,
	DeviceTypeDefinition,
	DomainDefinition,
	RegistryChange,
	ClassroomProfile,
	ClassroomRole,
	InvitationStatus,
	Parameter,
	DeviceType,
	Lab,
	ObservationTemplate,
	Reading,
	User
} from '#lib/domain/types.ts';
import type { StoredAlertRule } from '#lib/domain/alerts.ts';
import type { CoreParameters } from '#lib/core-readings.ts';
import { createSeed } from './seed';

/**
 * The mock "backend" state. It is seeded on first load and persisted to localStorage so
 * sign-ups and edits survive a reload. Outside the browser (unit tests) it lives in memory.
 */

export interface StoredChallenge {
	requestId: string;
	phone: string;
	code: string;
	purpose: 'sign_in' | 'verify_phone';
	userId: string | null;
	attempts: number;
	resendAvailableAt: string;
	expiresAt: string;
}

export interface StoredClassroom {
	id: string;
	name: string;
	institutionName: string;
	code: string;
	setup: 'self_service' | 'admin_assisted';
	createdBy: string;
	createdAt: string;
	/** Licence, credit and restrictions every export of its data carries (AP-14). */
	terms: DataTerms;
	/** Its stage in `siteContent.stages`, shown on the public network map (AP-20). */
	stageId: string;
	/** Its region in `siteContent.regions`, for the public directory (AP-28); null until set. */
	regionId: string | null;
	/** Readings observed up to here are released to the public; null while none are. */
	releasedThrough: string | null;
	/** What its public profile shows (AP-29). */
	profile: ClassroomProfile;
}

export interface StoredMembership {
	classroomId: string;
	userId: string;
	role: ClassroomRole;
	joinedAt: string;
}

export interface StoredInvitation {
	id: string;
	/** Secret part of the invite link. */
	token: string;
	classroomId: string;
	email: string;
	role: ClassroomRole;
	status: InvitationStatus;
	invitedBy: string;
	createdAt: string;
	acceptedBy: string | null;
}

/** Stand-in for SAM's source registry: classroom codes are registered as source identifiers. */
export interface StoredSamSource {
	sourceId: string;
	kind: 'classroom' | 'device';
	registeredAt: string;
}

export interface StoredDevice {
	id: string;
	labId: string;
	name: string;
	type: DeviceType;
	identifier: string;
	registeredAt: string;
}

type StoredRecord = Omit<Reading, 'creditedTo' | 'recordedByParticipantId'> & {
	samSourceId: string;
	/** Who recorded it, when they opted in to credit. Resolved to their current name on read. */
	creditedToUserId: string | null;
	/** Who recorded or synced it, credited or not, so contributors can be counted. Null for a
	 * device that uploads on its own. */
	recordedByUserId: string | null;
	/** The device's ID for a record synced from its outbox; a retry finds it again. */
	clientId?: string;
	clockFlags?: ClockFlag[];
};

/**
 * A minor's guardian consent: one per minor. A link (AP-04) is replaced when they ask again; a
 * paper sheet recorded by the classroom (AP-38) updates the same record and keeps any link.
 */
export interface StoredConsent {
	userId: string;
	/** The guardian link's secret; null when no link was ever sent. */
	token: string | null;
	/** How the current status was decided or recorded. */
	method: ConsentMethod;
	/** The day on a paper sheet, YYYY-MM-DD. */
	signedOn: string | null;
	/** The head or facilitator who recorded a paper sheet. */
	recordedBy: string | null;
	guardianName: string;
	guardianContact: string;
	status: Exclude<ConsentStatus, 'not_needed'>;
	requestedAt: string;
	decidedAt: string | null;
}

/** Stand-in for attribute assertions sent to SAM (AP-36); SAM enforces them. */
export interface StoredAssertion {
	userId: string;
	attribute: 'consent';
	value: 'given' | 'withdrawn';
	assertedAt: string;
}

/** An application to host a classroom (AP-30), with its status link's token. */
export interface StoredHostApplication extends HostApplication {
	token: string;
}

export interface MockDbState {
	version: number;
	users: User[];
	/** Mock Google accounts available in the sign-in picker. */
	googleAccounts: { sub: string; name: string; email: string; avatarUrl: string | null }[];
	/** Maps a Google `sub` to a user ID. */
	googleLinks: Record<string, string>;
	challenges: StoredChallenge[];
	/** Stand-in for the HTTP-only session cookie the real backend will issue. */
	session: { token: string; userId: string } | null;
	parameters: Parameter[];
	domains: DomainDefinition[];
	deviceTypes: DeviceTypeDefinition[];
	/** The SAM registry release being served; calculations cite it. */
	registryVersion: { version: string; publishedAt: string };
	/** Registry changes, newest first. */
	registryChanges: RegistryChange[];
	observationTemplates: ObservationTemplate[];
	/** Platform default alert levels (AP-08). */
	alertDefaults: AlertRule[];
	/** Alert levels set for one device, or for a sensor type across a classroom. */
	alertRules: StoredAlertRule[];
	/** Readings received from devices. Each was also forwarded to SAM under `samSourceId`. */
	readings: (StoredRecord & {
		/** Set when it was entered by hand through an observation template. */
		templateId?: string;
	})[];
	/** Field observations: no device, sent to SAM under the classroom's source. */
	observations: (Omit<StoredRecord, 'deviceId'> & { deviceId: null; templateId: string })[];
	classrooms: StoredClassroom[];
	/** The public site's words and structure (AP-20). */
	siteContent: SiteContent;
	/** Which values feed the public core readings strip. */
	coreParameters: CoreParameters;
	/** What "What's happening now?" shows on a station (AP-43). */
	nowTiles: NowTileConfig[];
	/** New classrooms start with these; `{classroom}` and `{institution}` are filled in. */
	defaultDataTerms: DataTerms;
	memberships: StoredMembership[];
	invitations: StoredInvitation[];
	samSources: StoredSamSource[];
	labs: Lab[];
	devices: StoredDevice[];
	guardianConsents: StoredConsent[];
	samAssertions: StoredAssertion[];
	/** Readiness questions for hosting a classroom (AP-30), in order. */
	hostQuestions: HostQuestion[];
	hostApplications: StoredHostApplication[];
	/** How much of a minor's position is kept: decimal places of latitude and longitude. */
	privacy: { minorCoordinateDecimals: number };
}

const STORAGE_KEY = 'dagrouper:mock-db';
// Bump when the shape changes: older saved data is dropped and reseeded.
const VERSION = 34;
const RELOAD_KEY = 'dagrouper:reloaded-for-version';

let memory: MockDbState | null = null;
/**
 * Set when another tab saved the data with a newer release: this tab runs old code and must not
 * overwrite that data with its own shape.
 */
let staleRelease = false;

function storage(): Storage | null {
	try {
		return typeof localStorage === 'undefined' ? null : localStorage;
	} catch {
		return null;
	}
}

export function readDb(): MockDbState {
	if (memory) return memory;
	const raw = storage()?.getItem(STORAGE_KEY);
	if (raw) {
		try {
			const parsed = JSON.parse(raw) as MockDbState;
			if (parsed.version === VERSION) return (memory = parsed);
			if (parsed.version > VERSION) {
				staleRelease = true;
				reloadOnce(parsed.version);
			}
		} catch {
			// Corrupt or old data: fall through and reseed.
		}
	}
	memory = { version: VERSION, ...createSeed() };
	writeDb(memory);
	return memory;
}

/** Loads the newer release once. If the reload still serves old code, carry on from memory. */
function reloadOnce(version: number) {
	if (typeof location === 'undefined') return;
	try {
		if (sessionStorage.getItem(RELOAD_KEY) === String(version)) return;
		sessionStorage.setItem(RELOAD_KEY, String(version));
	} catch {
		return;
	}
	location.reload();
}

function versionOf(raw: string | null): number | null {
	try {
		const version = raw && (JSON.parse(raw) as Partial<MockDbState>).version;
		return typeof version === 'number' ? version : null;
	} catch {
		return null;
	}
}

export function writeDb(state: MockDbState): void {
	memory = state;
	if (staleRelease) return;
	try {
		storage()?.setItem(STORAGE_KEY, JSON.stringify(state));
	} catch {
		// Storage full or blocked: keep working from memory.
	}
}

/**
 * Calls `onChange` when another tab in this browser changes the shared mock data, after
 * dropping the cached copy so the next read sees the change. Returns an unsubscribe function.
 */
export function watchOtherTabs(onChange: () => void): () => void {
	if (typeof window === 'undefined') return () => {};
	const listener = (event: StorageEvent) => {
		if (event.key !== STORAGE_KEY && event.key !== null) return;
		// A tab still running an older release reseeds the data in its own shape, without the
		// session. Reseeding in reply would sign both tabs out on every write, so keep ours.
		const version = versionOf(event.newValue);
		if (version !== null && version < VERSION) return;
		memory = null;
		onChange();
	};
	window.addEventListener('storage', listener);
	return () => window.removeEventListener('storage', listener);
}

/** Applies a change and persists it. */
export function updateDb<T>(fn: (state: MockDbState) => T): T {
	const state = readDb();
	const result = fn(state);
	writeDb(state);
	return result;
}

/** Drops every change and restores the seed data. */
export function resetDb(): void {
	memory = null;
	storage()?.removeItem(STORAGE_KEY);
}

export function newId(prefix: string): string {
	const random =
		typeof crypto !== 'undefined' && 'randomUUID' in crypto
			? crypto.randomUUID()
			: Math.random().toString(36).slice(2);
	return `${prefix}_${random}`;
}
