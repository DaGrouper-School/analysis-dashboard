import type {
	ClockFlag,
	Observation,
	ObservationInput,
	SyncBatch,
	SyncBatchResult
} from '#lib/domain/types.ts';

/**
 * The device's outbox (AP-16, offline recording): every observation is saved here first, with an
 * ID made on the device, and stays until the backend has stored it. Kept apart from the app's
 * data so it survives sign-out, reloads and lost connections.
 */

export type OutboxStatus = 'pending' | 'synced' | 'failed';

export interface OutboxEntry {
	/** Made on the device when saved. Every attempt sends the same one. */
	clientId: string;
	/** Whose observation it is: only they sync it and see it. */
	userId: string;
	observation: ObservationInput;
	credit: boolean;
	/** The device clock when it was saved. */
	savedAt: string;
	status: OutboxStatus;
	attempts: number;
	lastAttemptAt: string | null;
	/** Why the last attempt did not go through: the server's reason, or no connection. */
	error: string | null;
	/** Field-level reasons when the server refused it. */
	fields: Record<string, string>;
	clockFlags: ClockFlag[];
	/** Server clock minus the device clock at the last attempt, in ms. */
	clockOffsetMs: number;
	/** The stored record, once synced. */
	stored: Observation | null;
}

export const OUTBOX_KEY = 'dagrouper:outbox';
/** Synced entries kept per user for the status list; older ones are dropped. */
export const KEEP_SYNCED = 20;
/** The backend's limit on one batch. */
export const BATCH_SIZE = 100;

export function newEntry(
	userId: string,
	observation: ObservationInput,
	credit: boolean,
	at: Date,
	clientId: string
): OutboxEntry {
	return {
		clientId,
		userId,
		observation: structuredClone(observation),
		credit,
		savedAt: at.toISOString(),
		status: 'pending',
		attempts: 0,
		lastAttemptAt: null,
		error: null,
		fields: {},
		clockFlags: [],
		clockOffsetMs: 0,
		stored: null
	};
}

/** A device-made ID: a UUID where the browser has one. */
export function newClientId(): string {
	if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID();
	return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
}

/** The user's pending entries, oldest first, as one batch of at most `BATCH_SIZE`. */
export function nextBatch(entries: OutboxEntry[], userId: string, sentAt: Date): SyncBatch {
	return {
		sentAt: sentAt.toISOString(),
		records: entries
			.filter((e) => e.userId === userId && e.status === 'pending')
			.sort((a, b) => a.savedAt.localeCompare(b.savedAt))
			.slice(0, BATCH_SIZE)
			.map((e) => ({ clientId: e.clientId, observation: e.observation, credit: e.credit }))
	};
}

/** Each record's own result: stored or already stored is synced, refused is failed. */
export function applyResults(
	entries: OutboxEntry[],
	result: SyncBatchResult,
	at: Date
): OutboxEntry[] {
	const byId = new Map(result.results.map((r) => [r.clientId, r]));
	return entries.map((entry) => {
		const r = byId.get(entry.clientId);
		if (!r) return entry;
		const synced = r.outcome !== 'rejected';
		return {
			...entry,
			status: synced ? 'synced' : 'failed',
			attempts: entry.attempts + 1,
			lastAttemptAt: at.toISOString(),
			error: synced ? null : (r.error ?? 'Not accepted.'),
			fields: synced ? {} : { ...r.fields },
			clockFlags: [...r.clockFlags],
			clockOffsetMs: result.clockOffsetMs,
			stored: r.observation
		};
	});
}

/** An attempt that never reached the server: the entries stay pending, with the reason. */
export function markUnsent(
	entries: OutboxEntry[],
	clientIds: string[],
	error: string,
	at: Date
): OutboxEntry[] {
	const ids = new Set(clientIds);
	return entries.map((e) =>
		ids.has(e.clientId)
			? { ...e, attempts: e.attempts + 1, lastAttemptAt: at.toISOString(), error }
			: e
	);
}

/** Sends a failed entry again, unchanged and with the same ID. */
export function requeue(entries: OutboxEntry[], clientId: string): OutboxEntry[] {
	return entries.map((e) =>
		e.clientId === clientId && e.status === 'failed'
			? { ...e, status: 'pending', error: null, fields: {} }
			: e
	);
}

/** Drops all but each user's `KEEP_SYNCED` newest synced entries. Unsynced ones always stay. */
export function prune(entries: OutboxEntry[]): OutboxEntry[] {
	const keep = new Set<string>();
	const seen = new Map<string, number>();
	for (const e of [...entries].sort((a, b) => b.savedAt.localeCompare(a.savedAt))) {
		if (e.status !== 'synced') continue;
		const n = seen.get(e.userId) ?? 0;
		if (n < KEEP_SYNCED) keep.add(e.clientId);
		seen.set(e.userId, n + 1);
	}
	return entries.filter((e) => e.status !== 'synced' || keep.has(e.clientId));
}

/** What a clock flag means, for the person who recorded it. */
export function clockMessage(flag: ClockFlag, offsetMs: number): string {
	if (flag === 'observed_after_sent') {
		return 'Its time is later than when your device sent it. Check the time on the observation.';
	}
	const minutes = Math.max(1, Math.round(Math.abs(offsetMs) / 60_000));
	const span = minutes >= 120 ? `${Math.round(minutes / 60)} hours` : `${minutes} min`;
	return `Your device clock was ${span} ${offsetMs > 0 ? 'behind' : 'ahead'} when this was sent, so its time may be wrong.`;
}

function isEntry(value: unknown): value is OutboxEntry {
	const e = value as Partial<OutboxEntry> | null;
	return (
		!!e &&
		typeof e.clientId === 'string' &&
		typeof e.userId === 'string' &&
		typeof e.savedAt === 'string' &&
		(e.status === 'pending' || e.status === 'synced' || e.status === 'failed') &&
		!!e.observation
	);
}

/** The saved outbox; anything unreadable is left out. */
export function parseOutbox(raw: string | null): OutboxEntry[] {
	if (!raw) return [];
	try {
		const parsed: unknown = JSON.parse(raw);
		return Array.isArray(parsed) ? parsed.filter(isEntry) : [];
	} catch {
		return [];
	}
}
