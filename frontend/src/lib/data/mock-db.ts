import type {
	ClassroomRole,
	InvitationStatus,
	Parameter,
	DeviceType,
	Lab,
	Reading,
	User
} from '#lib/domain/types.ts';
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
	/** Readings received from devices. Each was also forwarded to SAM under `samSourceId`. */
	readings: (Reading & { samSourceId: string })[];
	classrooms: StoredClassroom[];
	memberships: StoredMembership[];
	invitations: StoredInvitation[];
	samSources: StoredSamSource[];
	labs: Lab[];
	devices: StoredDevice[];
}

const STORAGE_KEY = 'dagrouper:mock-db';
// Bump when the shape changes: older saved data is dropped and reseeded.
const VERSION = 6;

let memory: MockDbState | null = null;

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
		} catch {
			// Corrupt or old data: fall through and reseed.
		}
	}
	memory = { version: VERSION, ...createSeed() };
	writeDb(memory);
	return memory;
}

export function writeDb(state: MockDbState): void {
	memory = state;
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
