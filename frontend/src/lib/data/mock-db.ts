import type {
	AlertRule,
	DeviceTypeDefinition,
	DomainDefinition,
	RegistryChange,
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

type StoredRecord = Omit<Reading, 'creditedTo'> & {
	samSourceId: string;
	/** Who recorded it, when they opted in to credit. Resolved to their current name on read. */
	creditedToUserId: string | null;
};

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
	memberships: StoredMembership[];
	invitations: StoredInvitation[];
	samSources: StoredSamSource[];
	labs: Lab[];
	devices: StoredDevice[];
}

const STORAGE_KEY = 'dagrouper:mock-db';
// Bump when the shape changes: older saved data is dropped and reseeded.
const VERSION = 18;
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
