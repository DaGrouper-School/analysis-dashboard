import type { ObservationInput } from '#lib/domain/types.ts';
import {
	OUTBOX_KEY,
	applyResults,
	markUnsent,
	newClientId,
	newEntry,
	nextBatch,
	parseOutbox,
	prune,
	requeue,
	type OutboxEntry
} from '#lib/outbox.ts';
import { ServiceError, services } from '#lib/services/index.ts';

const OFFLINE_KEY = 'dagrouper:simulate-offline';

function read(key: string): string | null {
	try {
		return localStorage.getItem(key);
	} catch {
		return null;
	}
}

function write(key: string, value: string | null) {
	try {
		if (value === null) localStorage.removeItem(key);
		else localStorage.setItem(key, value);
	} catch {
		// Storage full or blocked: keep working from memory for this visit.
	}
}

/**
 * The device outbox, live: what is waiting, whether the device is online, and syncing. Syncs run
 * one at a time, when the app opens, when the connection comes back, after each new
 * observation, and on demand.
 */
class Outbox {
	entries = $state<OutboxEntry[]>(parseOutbox(read(OUTBOX_KEY)));
	userId = $state<string | null>(null);
	browserOnline = $state(typeof navigator === 'undefined' ? true : navigator.onLine);
	/** Demo only: behave as if the connection were gone. */
	simulatedOffline = $state(read(OFFLINE_KEY) === '1');
	syncing = $state(false);

	online = $derived(this.browserOnline && !this.simulatedOffline);
	/** The signed-in user's entries, newest first. */
	mine = $derived(
		this.entries
			.filter((e) => e.userId === this.userId)
			.sort((a, b) => b.savedAt.localeCompare(a.savedAt))
	);
	/** Saved but not yet stored: pending or failed. */
	waiting = $derived(this.mine.filter((e) => e.status !== 'synced').length);

	#queue: Promise<void> = Promise.resolve();

	#set(next: OutboxEntry[]) {
		this.entries = prune(next);
		write(OUTBOX_KEY, JSON.stringify(this.entries));
	}

	/** Follows the connection and other tabs for this user, and syncs what is waiting. */
	start(userId: string): () => void {
		this.userId = userId;
		const online = () => {
			this.browserOnline = true;
			void this.sync();
		};
		const offline = () => (this.browserOnline = false);
		const storage = (event: StorageEvent) => {
			if (event.key === OUTBOX_KEY || event.key === null) {
				this.entries = parseOutbox(read(OUTBOX_KEY));
			}
		};
		window.addEventListener('online', online);
		window.addEventListener('offline', offline);
		window.addEventListener('storage', storage);
		void this.sync();
		return () => {
			window.removeEventListener('online', online);
			window.removeEventListener('offline', offline);
			window.removeEventListener('storage', storage);
		};
	}

	setSimulatedOffline(value: boolean) {
		this.simulatedOffline = value;
		write(OFFLINE_KEY, value ? '1' : null);
		if (!value) void this.sync();
	}

	/** Saves the observation on the device first, then tries to send it. Returns its entry. */
	async save(observation: ObservationInput, credit: boolean): Promise<OutboxEntry> {
		if (!this.userId) throw new ServiceError('unauthenticated', 'Please sign in again.');
		const entry = newEntry(this.userId, observation, credit, new Date(), newClientId());
		this.#set([...this.entries, entry]);
		await this.sync();
		return this.entries.find((e) => e.clientId === entry.clientId) ?? entry;
	}

	/** Sends everything pending, in batches. Calls made while one runs wait their turn. */
	sync(): Promise<void> {
		this.#queue = this.#queue.then(() => this.#run());
		return this.#queue;
	}

	async retry(clientId: string) {
		this.#set(requeue(this.entries, clientId));
		await this.sync();
	}

	discard(clientId: string) {
		this.#set(this.entries.filter((e) => e.clientId !== clientId));
	}

	async #run() {
		const userId = this.userId;
		if (!userId || !this.online) return;
		this.syncing = true;
		const tried: string[] = [];
		try {
			for (;;) {
				const batch = nextBatch(
					this.entries.filter((e) => !tried.includes(e.clientId)),
					userId,
					new Date()
				);
				if (!batch.records.length || !this.online) return;
				const ids = batch.records.map((r) => r.clientId);
				tried.push(...ids);
				try {
					const result = await services.labs.syncObservations(batch);
					this.#set(applyResults(this.entries, result, new Date()));
				} catch (e) {
					// The batch never got through: everything in it stays pending for the next try.
					const reason =
						e instanceof ServiceError && e.code !== 'network'
							? e.message
							: 'Could not reach the server. It will try again when you are online.';
					this.#set(markUnsent(this.entries, ids, reason, new Date()));
					return;
				}
			}
		} finally {
			this.syncing = false;
		}
	}
}

export const outbox = new Outbox();
