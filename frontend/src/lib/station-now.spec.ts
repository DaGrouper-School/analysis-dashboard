import { describe, expect, it } from 'vitest';
import type { NowTileConfig, Parameter, PublicValue } from '#lib/domain/types.ts';
import {
	compassPoint,
	formatCoordinates,
	initials,
	nowTiles,
	placeLabel,
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
