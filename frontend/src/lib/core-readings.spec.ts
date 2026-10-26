import { describe, expect, it } from 'vitest';
import { coreReadings, type CoreParameters } from './core-readings';

const parameters: CoreParameters = {
	rainDaily: ['precip_accum_local_day', 'rain_gauge_daily_accumulation'],
	airTemperature: ['air_temperature']
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

	it('takes each source’s day total, averages the sources, and sums the last seven days', () => {
		const core = coreReadings(records, '2026-10-20T08:00:00.000Z', parameters);
		// Station 4 mm, gauge 6 mm: 5 mm today. Plus 2 mm on the 19th; the 12th is outside.
		expect(core).toMatchObject({
			rainToday: 5,
			rainLast7Days: 7,
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
