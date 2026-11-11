import { describe, expect, it } from 'vitest';
import type { NowTileConfig, Parameter, PublicValue } from '#lib/domain/types.ts';
import {
	compassPoint,
	formatCoordinates,
	initials,
	last24Hours,
	nowTiles,
	placeLabel,
	readingTiles,
	timeAgo
} from './station-now.ts';

describe('station now helpers (AP-43)', () => {
	it('names compass points', () => {
		expect(compassPoint(0)).toBe('north');
		expect(compassPoint(112)).toBe('east-southeast');
		expect(compassPoint(359)).toBe('north');
		expect(compassPoint(-90)).toBe('west');
	});

	it('makes short place labels', () => {
		expect(placeLabel('Trinity School, Addis Ababa', 'Ethiopia')).toBe('Addis Ababa, Ethiopia');
		expect(placeLabel(null, 'Somalia')).toBe('Somalia');
		expect(placeLabel('Nassau', 'Nassau')).toBe('Nassau');
	});

	it('makes initials and coordinates', () => {
		expect(initials('Bay Street School')).toBe('BS');
		expect(initials('Trinity School, Addis Ababa')).toBe('TS');
		expect(formatCoordinates({ lat: 9.0301, lng: 38.7401 })).toBe('9.03° N, 38.74° E');
		expect(formatCoordinates({ lat: 25.08, lng: -77.34 })).toBe('25.08° N, 77.34° W');
	});

	it('says how long ago', () => {
		const now = Date.parse('2026-10-20T12:00:00Z');
		expect(timeAgo('2026-10-20T12:00:00Z', now)).toBe('just now');
		expect(timeAgo('2026-10-20T11:55:00Z', now)).toBe('5 mins ago');
		expect(timeAgo('2026-10-20T11:00:00Z', now)).toBe('1 hour ago');
		expect(timeAgo('2026-10-17T12:00:00Z', now)).toBe('3 days ago');
	});

	it('builds tiles from the newest value, with captions and zero text', () => {
		const p = (id: string, unit: string) => ({ id, unit, valueType: 'number' }) as Parameter;
		const v = (parameterId: string, value: number, observedAt: string) =>
			({ parameterId, value, observedAt }) as PublicValue;
		const config: NowTileConfig[] = [
			{
				id: 'wind',
				label: 'Wind',
				icon: 'wind',
				parameterIds: ['wind_avg'],
				caption: { parameterIds: ['wind_direction'], text: 'From the {value}', format: 'compass' }
			},
			{
				id: 'rain',
				label: 'Rain today',
				icon: 'rain',
				parameterIds: ['rain_a', 'rain_b'],
				zeroText: 'No rain today'
			},
			{ id: 'pressure', label: 'Pressure', icon: 'pressure', parameterIds: ['pressure'] }
		];
		const tiles = nowTiles(
			config,
			[
				v('wind_avg', 2, '2026-10-20T05:00:00Z'),
				v('wind_avg', 3.2, '2026-10-20T06:00:00Z'),
				v('wind_direction', 90, '2026-10-20T06:00:00Z'),
				v('rain_b', 0, '2026-10-20T06:00:00Z')
			],
			[
				p('wind_avg', 'm/s'),
				p('wind_direction', '°'),
				p('rain_a', 'mm'),
				p('rain_b', 'mm'),
				p('pressure', 'hPa')
			]
		);
		expect(tiles[0]).toMatchObject({ value: 3.2, unit: 'm/s', caption: 'From the east' });
		expect(tiles[1]).toMatchObject({ parameterId: 'rain_b', value: 0, caption: 'No rain today' });
		expect(tiles[2]).toMatchObject({ value: null, unit: 'hPa', caption: null, observedAt: null });
	});
});

describe('last 24 hours', () => {
	it('keeps the latest reading of each hour, oldest first, and leaves quiet hours empty', () => {
		const at = (observedAt: string, value: number) => ({ observedAt, values: { t: value } });
		const hours = last24Hours(
			[
				at('2026-10-20T06:00:00.000Z', 20),
				at('2026-10-20T05:10:00.000Z', 18),
				at('2026-10-20T04:50:00.000Z', 19),
				at('2026-10-19T06:00:00.000Z', 5)
			],
			['t'],
			'2026-10-20T06:00:00.000Z'
		);
		expect(hours).toHaveLength(24);
		expect(hours.at(-1)).toBe(20);
		expect(hours.at(-2)).toBe(19);
		expect(hours[0]).toBeNull();
	});
});

describe('one device now', () => {
	it("shows the device's newest values, with its icon where a value has none", () => {
		const readings = [
			{ observedAt: '2026-10-20T06:00:00.000Z', values: { soil_moisture: 31, ph: 6.5 } },
			{ observedAt: '2026-10-20T05:00:00.000Z', values: { soil_moisture: 30, ph: 6.4 } }
		];
		const tiles = readingTiles(
			readings,
			[
				{
					id: 'soil_moisture',
					label: 'Soil moisture',
					unit: '%',
					valueType: 'number',
					value: 31,
					icon: 'droplet'
				},
				{ id: 'ph', label: 'pH', unit: '', valueType: 'number', value: 6.5 }
			],
			'soil'
		);
		expect(tiles.map((t) => [t.parameterId, t.icon, t.value, t.observedAt])).toEqual([
			['soil_moisture', 'droplet', 31, '2026-10-20T06:00:00.000Z'],
			['ph', 'soil', 6.5, '2026-10-20T06:00:00.000Z']
		]);
		expect(tiles[0].last24Hours.slice(-2)).toEqual([30, 31]);
	});
});
