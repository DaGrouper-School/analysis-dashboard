import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const KEY = 'dagrouper:mock-db';

/** A browser with one other tab writing to the same localStorage. */
function stubBrowser() {
	const items = new Map<string, string>();
	vi.stubGlobal('window', new EventTarget());
	vi.stubGlobal('localStorage', {
		getItem: (key: string) => items.get(key) ?? null,
		setItem: (key: string, value: string) => void items.set(key, value),
		removeItem: (key: string) => void items.delete(key)
	});
	/** What another tab does: save, then this tab hears about it. */
	const otherTabWrites = (value: object) => {
		const newValue = JSON.stringify(value);
		items.set(KEY, newValue);
		window.dispatchEvent(Object.assign(new Event('storage'), { key: KEY, newValue }));
	};
	return { items, otherTabWrites };
}

async function freshDb() {
	vi.resetModules();
	return import('./mock-db');
}

beforeEach(() => vi.unstubAllGlobals());
afterEach(() => vi.unstubAllGlobals());

describe('tabs on different releases', () => {
	it('keeps the session when a tab on an older release reseeds the data', async () => {
		const { otherTabWrites } = stubBrowser();
		const db = await freshDb();
		const version = db.readDb().version;
		db.updateDb((state) => (state.session = { token: 'sess_1', userId: 'usr_tesfaye' }));
		const onChange = vi.fn();
		db.watchOtherTabs(onChange);

		otherTabWrites({ version: version - 1, session: null });

		expect(onChange).not.toHaveBeenCalled();
		expect(db.readDb().session?.userId).toBe('usr_tesfaye');
	});

	it('picks up changes from a tab on the same release', async () => {
		const { otherTabWrites } = stubBrowser();
		const db = await freshDb();
		const state = db.readDb();
		const onChange = vi.fn();
		db.watchOtherTabs(onChange);

		otherTabWrites({ ...state, session: { token: 'sess_2', userId: 'usr_sarah' } });

		expect(onChange).toHaveBeenCalledOnce();
		expect(db.readDb().session?.userId).toBe('usr_sarah');
	});

	it('never overwrites data saved by a newer release', async () => {
		const { items } = stubBrowser();
		const db = await freshDb();
		const version = db.readDb().version;
		const newer = JSON.stringify({ version: version + 1, session: null });
		db.resetDb();
		items.set(KEY, newer);

		db.updateDb((state) => (state.session = { token: 'sess_3', userId: 'usr_tesfaye' }));

		expect(items.get(KEY)).toBe(newer);
	});
});
