import { describe, expect, it } from 'vitest';
import type { Parameter } from '#lib/domain/types.ts';
import { createSeed } from '#lib/data/seed.ts';
import { describeParameter, formatValue, keySummary, keyValues, timeAgo } from './key-values';

const parameters = new Map<string, Parameter>([
	[
		'air_temperature',
		{
			id: 'air_temperature',
			label: 'Air temperature',
			definition: '',
			unit: '°C',
			domain: 'AIR',
			source: 'weather_station'
		}
	],
	[
		'precip_accum_local_day',
		{
			id: 'precip_accum_local_day',
			label: 'Rain since midnight',
			definition: '',
			unit: 'mm',
			domain: 'RAIN',
			source: 'weather_station'
		}
	]
]);

describe('key values per sensor type', () => {
	it('shows soil values for a soil sensor, and nothing about rain', () => {
		const values = { soil_moisture: 31, soil_temperature: 18.4, precip_accum_local_day: 2 };
		expect(keyValues('soil_sensor', values, parameters)).toEqual([
			{ id: 'soil_moisture', label: 'Soil moisture', unit: '%', valueType: 'number', value: 31 },
			{
				id: 'soil_temperature',
				label: 'Soil temperature',
				unit: '°C',
				valueType: 'number',
				value: 18.4
			}
		]);
	});

	it('leads a weather station with its headline values, units from the registry', () => {
		const values = { relative_humidity: 60, precip_accum_local_day: 0, air_temperature: 27 };
		expect(keySummary('weather_station', values, parameters)).toBe(
			'Temperature 27°C · Rain today 0 mm'
		);
	});

	it('falls back to whatever an unknown device sends', () => {
		expect(keySummary('other', { light_level: 4 }, parameters)).toBe('Light level 4');
	});

	it('formats values for reading at a glance', () => {
		expect(formatValue(1013.46, 'hPa')).toBe('1013 hPa');
		expect(formatValue(undefined)).toBe('—');
	});

	it('describes parameters the registry lacks from the sensor types', () => {
		expect(describeParameter('soil_moisture', parameters)).toEqual({
			label: 'Soil moisture',
			unit: '%',
			valueType: 'number'
		});
		expect(describeParameter('air_temperature', parameters).label).toBe('Air temperature');
		expect(describeParameter('light_level', parameters)).toEqual({
			label: 'Light level',
			unit: '',
			valueType: 'number'
		});
	});
});

describe('times', () => {
	it('shows a time value as a date in the reader’s language and time zone', () => {
		const at = 1792475220;
		const expected = new Intl.DateTimeFormat(undefined, {
			dateStyle: 'medium',
			timeStyle: 'short'
		}).format(new Date(at * 1000));
		expect(formatValue(at, 's', 'time')).toBe(expected);
		expect(formatValue(at, 's')).toBe('1792475220 s');
	});

	it('marks the last lightning strike as a time in the registry', () => {
		const strike = createSeed().parameters.find((p) => p.id === 'lightning_strike_last_epoch');
		expect(strike?.valueType).toBe('time');
	});

	it('says how long ago a time was', () => {
		const now = Date.UTC(2026, 9, 20, 8);
		const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });
		expect(timeAgo(now / 1000 - 2 * 3600 - 13 * 60, now)).toBe(rtf.format(-2, 'hour'));
		expect(timeAgo(now / 1000 - 3 * 86400, now)).toBe(rtf.format(-3, 'day'));
		expect(timeAgo(now / 1000 - 20, now)).toBe(rtf.format(0, 'minute'));
	});
});
