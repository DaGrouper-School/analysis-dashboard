import { describe, expect, it } from 'vitest';
import type { Parameter } from '#lib/domain/types.ts';
import { describeParameter, formatValue, keySummary, keyValues } from './key-values';

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
			{ id: 'soil_moisture', label: 'Soil moisture', unit: '%', value: 31 },
			{ id: 'soil_temperature', label: 'Soil temperature', unit: '°C', value: 18.4 }
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
			unit: '%'
		});
		expect(describeParameter('air_temperature', parameters).label).toBe('Air temperature');
		expect(describeParameter('light_level', parameters)).toEqual({
			label: 'Light level',
			unit: ''
		});
	});
});
