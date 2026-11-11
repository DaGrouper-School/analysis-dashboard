import { describe, expect, it } from 'vitest';
import { coreReadings, stationCore, type CoreParameters } from './core-readings';

const parameters: CoreParameters = {
	rainDaily: ['precip_accum_local_day', 'rain_gauge_daily_accumulation'],
	airTemperature: ['air_temperature'],
	rainTolerance: 2,
	rainReference: 'rain_gauge_daily_accumulation'
};

const at = (observedAt: string, values: Record<string, number | string>, deviceId = 'ws') => ({
	observedAt,
	receivedAt: observedAt.replace('00.000Z', '30.000Z'),
	deviceId,
	values
});

const records = [
	at('2026-10-20T08:00:00.000Z', { precip_accum_local_day: 4, air_temperature: 27 }),
	at('2026-10-20T06:00:00.000Z', { precip_accum_local_day: 3, air_temperature: 25 }),
	at('2026-10-20T07:00:00.000Z', { rain_gauge_daily_accumulation: 6 }, 'gauge'),
	at('2026-10-19T08:00:00.000Z', { precip_accum_local_day: 2, air_temperature: 24 }),
	at('2026-10-12T08:00:00.000Z', { precip_accum_local_day: 9 })
];

describe('core readings', () => {
	it('counts only released data and says when newer data is in review', () => {
		const core = coreReadings(records, '2026-10-20T06:00:00.000Z', parameters);
		expect(core).toMatchObject({
			day: '2026-10-20',
			rainToday: 3,
			airTemperature: 25,
			lastSync: '2026-10-20T06:00:30.000Z',
			review: 'in_review'
		});
	});

	it('takes the gauge’s day total over the station’s, never an average, and sums seven days', () => {
		const core = coreReadings(records, '2026-10-20T08:00:00.000Z', parameters);
		// Station 4 mm, gauge 6 mm: the gauge is the reference, 6 mm today. Plus the station's
		// 2 mm on the 19th, with no gauge reading; the 12th is outside.
		expect(core).toMatchObject({
			rainToday: 6,
			rainLast7Days: 8,
			rainDays: 2,
			airTemperature: 27,
			review: 'released'
		});
	});

	it('leaves gaps as null, never zero', () => {
		const noRain = coreReadings(
			[at('2026-10-20T08:00:00.000Z', { air_temperature: 27 })],
			'2026-10-20T08:00:00.000Z',
			parameters
		);
		expect(noRain).toMatchObject({ rainToday: null, rainLast7Days: null, rainDays: 0 });
		expect(coreReadings(records, null, parameters)).toEqual({
			day: null,
			rainToday: null,
			rainLast7Days: null,
			rainDays: 0,
			airTemperature: null,
			lastSync: null,
			review: 'not_released'
		});
		expect(coreReadings([], '2026-10-20T08:00:00.000Z', parameters)).toMatchObject({
			lastSync: null,
			review: 'released'
		});
	});
});

describe('station core detail', () => {
	it('gives each core reading its review state, sources and spread', () => {
		const core = stationCore(records, '2026-10-20T07:00:00.000Z', parameters);
		// Released through 07:00: the station's 06:00 total and the gauge's 07:00 one.
		expect(core.rain).toMatchObject({ review: 'in_review', sources: 2, spread: 3 });
		// The gauge is the reference and comes first; 3 mm apart is past the 2 mm tolerance.
		expect(core.rain.bySource.map((s) => [s.sourceId, s.value, s.reference])).toEqual([
			['gauge', 6, true],
			['ws', 3, false]
		]);
		expect(core.rain.week).toHaveLength(7);
		expect(core.rain.week.at(-1)).toEqual({
			day: '2026-10-20',
			value: 6,
			reference: true,
			flagged: true
		});
		expect(core.temperature).toMatchObject({
			review: 'in_review',
			sources: 1,
			spread: null,
			low: 25,
			high: 25
		});
	});

	it('says released when nothing newer is waiting, and not released before any release', () => {
		expect(stationCore(records, '2026-10-21T00:00:00.000Z', parameters).temperature).toMatchObject({
			review: 'released',
			low: 25,
			high: 27
		});
		expect(stationCore(records, null, parameters).rain).toMatchObject({
			review: 'not_released',
			sources: 0,
			spread: null
		});
	});
});
