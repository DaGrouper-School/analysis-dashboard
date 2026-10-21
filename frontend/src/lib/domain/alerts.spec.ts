import { describe, expect, it } from 'vitest';
import { alertsFor, thresholdStatus } from './alerts';

describe('default alert thresholds', () => {
	it('raises one alert per parameter, the most severe one', () => {
		expect(alertsFor({ values: { uv: 9.1 } })).toEqual([
			expect.objectContaining({ parameterId: 'uv', level: 'warning', title: 'Very high UV' })
		]);
		expect(alertsFor({ values: { uv: 6.8 } })[0]).toMatchObject({ level: 'caution' });
	});

	it('stays quiet for normal readings and ignores text values', () => {
		expect(
			alertsFor({ values: { uv: 3.4, air_temperature: 27, pressure_trend: 'steady' } })
		).toEqual([]);
	});

	it('flags dry soil and heavy rain', () => {
		const titles = alertsFor({
			values: { soil_moisture: 12, rain_gauge_daily_accumulation: 61 }
		}).map((a) => a.title);
		expect(titles).toEqual(['Heavy rain', 'Dry soil']);
	});

	it('rates a value against its threshold for a badge', () => {
		expect(thresholdStatus('uv', 9.1)).toEqual({ level: 'warning', title: 'Very high UV' });
		expect(thresholdStatus('uv', 3)).toEqual({ level: 'normal', title: 'Normal' });
		expect(thresholdStatus('relative_humidity', 60)).toBeNull();
		expect(thresholdStatus('uv', undefined)).toBeNull();
	});
});
