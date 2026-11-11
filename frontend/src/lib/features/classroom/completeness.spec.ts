import { describe, expect, it } from 'vitest';
import { monthCompleteness } from '#lib/features/classroom/completeness.ts';

const domainOf = (id: string) =>
	id.startsWith('soil') ? { id: 'SOIL', label: 'Soil', icon: 'soil' } : null;
const at = (observedAt: string, values: Record<string, number | string>) => ({
	observedAt,
	values
});

describe('month completeness', () => {
	it('counts each expected day once and keeps gaps as gaps', () => {
		const records = [
			at('2026-10-01T08:00:00.000Z', { rain: 1 }),
			at('2026-10-01T18:00:00.000Z', { rain: 2 }),
			at('2026-10-03T08:00:00.000Z', { rain: 0 }),
			// Outside the month so far: ignored.
			at('2026-09-30T08:00:00.000Z', { rain: 4 }),
			at('2026-10-05T08:00:00.000Z', { rain: 4 })
		];
		const result = monthCompleteness(
			records,
			'2026-10-04',
			[{ parameterId: 'rain', readingsPerDay: 1 }],
			domainOf
		);
		expect(result.month).toBe('2026-10');
		expect(result.domains).toEqual([
			{
				domain: null,
				received: 2,
				expected: 4,
				days: [
					{ day: '2026-10-01', status: 'complete' },
					{ day: '2026-10-02', status: 'missing' },
					{ day: '2026-10-03', status: 'complete' },
					{ day: '2026-10-04', status: 'missing' }
				]
			}
		]);
	});

	it('marks a day partial when only some daily values came in', () => {
		const records = [at('2026-10-01T08:00:00.000Z', { soil_moisture: 30 })];
		const [soil] = monthCompleteness(
			records,
			'2026-10-01',
			[
				{ parameterId: 'soil_moisture', readingsPerDay: 1 },
				{ parameterId: 'soil_temperature', readingsPerDay: 1 }
			],
			domainOf
		).domains;
		expect(soil).toMatchObject({ received: 1, expected: 2, days: [{ status: 'partial' }] });
	});

	it('counts weekly values by week and gives them no day strip', () => {
		const records = [at('2026-10-02T08:00:00.000Z', { soil_moisture: 30 })];
		const [soil] = monthCompleteness(
			records,
			'2026-10-10',
			[{ parameterId: 'soil_moisture', readingsPerDay: 1 / 7 }],
			domainOf
		).domains;
		expect(soil).toMatchObject({ received: 1, expected: 2, days: [] });
	});

	it('uses the fastest rate any site asks for', () => {
		const records = [at('2026-10-01T08:00:00.000Z', { rain: 1 })];
		const [rain] = monthCompleteness(
			records,
			'2026-10-01',
			[
				{ parameterId: 'rain', readingsPerDay: 1 },
				{ parameterId: 'rain', readingsPerDay: 24 }
			],
			domainOf
		).domains;
		expect(rain).toMatchObject({ received: 1, expected: 24, days: [{ status: 'partial' }] });
	});
});
