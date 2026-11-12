import { describe, expect, it } from 'vitest';
import type { MockDbState } from './mock-db';
import { createSeed } from './seed';
import { rollSeedForward } from './seed-clock';

const DAY = 86_400_000;

function seeded(): MockDbState {
	return { version: 0, ...createSeed() };
}

describe('rollSeedForward', () => {
	it('moves the seeded history forward by whole days, so it ends today', () => {
		const state = seeded();
		const latest = Date.parse(state.seedClock.latest);
		const first = state.readings[0].observedAt;
		const now = latest + 23 * DAY + 5 * 3_600_000;

		expect(rollSeedForward(state, now)).toBe(true);
		expect(state.seedClock.latest).toBe(new Date(latest + 23 * DAY).toISOString());
		expect(state.readings[0].observedAt).toBe(new Date(Date.parse(first) + 23 * DAY).toISOString());
		expect(Math.max(...state.readings.map((r) => Date.parse(r.observedAt)))).toBeLessThanOrEqual(
			now
		);
		const trinity = state.classrooms.find((c) => c.id === 'cls_trinity')!;
		expect(trinity.releasedThrough).toBe('2026-11-12T06:00:00.000Z');
	});

	it('leaves records people added, and does nothing on the same day', () => {
		const state = seeded();
		const latest = Date.parse(state.seedClock.latest);
		const synced = {
			...state.readings[0],
			id: 'rdg_synced',
			observedAt: '2026-10-21T10:00:00.000Z'
		};
		state.readings.push(synced);

		expect(rollSeedForward(state, latest + 3_600_000)).toBe(false);
		rollSeedForward(state, latest + 2 * DAY);
		expect(state.readings.find((r) => r.id === 'rdg_synced')!.observedAt).toBe(
			'2026-10-21T10:00:00.000Z'
		);
	});
});
