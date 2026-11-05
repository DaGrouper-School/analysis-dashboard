import { describe, expect, it } from 'vitest';
import type { ObservationInput, SyncBatchResult } from '#lib/domain/types.ts';
import {
	BATCH_SIZE,
	KEEP_SYNCED,
	applyResults,
	clockMessage,
	markUnsent,
	newEntry,
	nextBatch,
	parseOutbox,
	prune,
	requeue
} from './outbox';

const input: ObservationInput = {
	templateId: 'tpl_rain_gauge',
	labId: 'lab_trinity',
	deviceId: 'dev_shashamane_gauge',
	observedAt: '2026-10-20T07:00:00.000Z',
	values: { rain_gauge_daily_accumulation: 4 },
	location: null
};
const at = (minute: number) => new Date(Date.UTC(2026, 9, 20, 8, minute));
const entry = (id: string, minute: number, userId = 'usr_a') =>
	newEntry(userId, input, false, at(minute), id);

const result = (
	results: (Partial<SyncBatchResult['results'][number]> & { clientId: string })[]
): SyncBatchResult => ({
	receivedAt: at(30).toISOString(),
	clockOffsetMs: 0,
	results: results.map((r) => ({
		outcome: 'stored',
		observation: null,
		error: null,
		fields: {},
		clockFlags: [],
		...r
	}))
});

describe('the device outbox', () => {
	it('saves new entries as pending, with the ID made on the device', () => {
		expect(entry('c1', 0)).toMatchObject({
			clientId: 'c1',
			status: 'pending',
			attempts: 0,
			savedAt: '2026-10-20T08:00:00.000Z'
		});
	});

	it("batches only the user's pending entries, oldest first, up to the limit", () => {
		const entries = [
			entry('late', 5),
			entry('early', 1),
			entry('other', 0, 'usr_b'),
			{ ...entry('done', 0), status: 'synced' as const },
			{ ...entry('bad', 0), status: 'failed' as const }
		];
		const batch = nextBatch(entries, 'usr_a', at(10));
		expect(batch.sentAt).toBe('2026-10-20T08:10:00.000Z');
		expect(batch.records.map((r) => r.clientId)).toEqual(['early', 'late']);
		const many = Array.from({ length: BATCH_SIZE + 5 }, (_, i) => entry(`c${i}`, 0));
		expect(nextBatch(many, 'usr_a', at(10)).records).toHaveLength(BATCH_SIZE);
	});

	it('applies each result to its own entry', () => {
		const entries = [entry('ok', 0), entry('dup', 1), entry('no', 2), entry('untouched', 3)];
		const next = applyResults(
			entries,
			result([
				{ clientId: 'ok' },
				{ clientId: 'dup', outcome: 'duplicate', clockFlags: ['device_clock_off'] },
				{ clientId: 'no', outcome: 'rejected', error: 'Nope', fields: { x: 'Bad' } }
			]),
			at(30)
		);
		expect(next.map((e) => [e.clientId, e.status, e.attempts])).toEqual([
			['ok', 'synced', 1],
			['dup', 'synced', 1],
			['no', 'failed', 1],
			['untouched', 'pending', 0]
		]);
		expect(next[1].clockFlags).toEqual(['device_clock_off']);
		expect(next[2]).toMatchObject({ error: 'Nope', fields: { x: 'Bad' } });
	});

	it('keeps entries pending when the batch never got through', () => {
		const next = markUnsent([entry('c1', 0)], ['c1'], 'No connection', at(30));
		expect(next[0]).toMatchObject({ status: 'pending', attempts: 1, error: 'No connection' });
	});

	it('retries a failed entry with the same ID', () => {
		const failed = { ...entry('c1', 0), status: 'failed' as const, error: 'Nope' };
		expect(requeue([failed], 'c1')[0]).toMatchObject({
			clientId: 'c1',
			status: 'pending',
			error: null
		});
	});

	it('drops old synced entries but never unsynced ones', () => {
		const synced = Array.from({ length: KEEP_SYNCED + 3 }, (_, i) => ({
			...entry(`s${i}`, i),
			status: 'synced' as const
		}));
		const kept = prune([...synced, entry('p', 0)]);
		expect(kept).toHaveLength(KEEP_SYNCED + 1);
		expect(kept.map((e) => e.clientId)).toContain('p');
		expect(kept.map((e) => e.clientId)).not.toContain('s0');
	});

	it('explains clock flags', () => {
		expect(clockMessage('device_clock_off', -40 * 60_000)).toBe(
			'Your device clock was 40 min ahead when this was sent, so its time may be wrong.'
		);
		expect(clockMessage('device_clock_off', 3 * 3_600_000)).toContain('3 hours behind');
		expect(clockMessage('observed_after_sent', 0)).toContain('later than when your device sent it');
	});

	it('reads back what was saved, skipping anything unreadable', () => {
		const saved = JSON.stringify([entry('c1', 0), { nope: true }]);
		expect(parseOutbox(saved).map((e) => e.clientId)).toEqual(['c1']);
		expect(parseOutbox('not json')).toEqual([]);
		expect(parseOutbox(null)).toEqual([]);
	});
});
