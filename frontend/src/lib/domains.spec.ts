import { describe, expect, it } from 'vitest';
import type { Parameter, Reading } from '#lib/domain/types.ts';
import { domainTabs, withinRange } from './domains';

const param = (id: string, domain: Parameter['domain']): Parameter => ({
	id,
	label: id,
	definition: '',
	unit: '',
	domain,
	source: 'weather_station'
});
const parameters = [
	param('air_temperature', 'AIR'),
	param('relative_humidity', 'AIR'),
	param('uv', 'SUN'),
	param('wind_avg', 'WIND')
];

function reading(observedAt: string, values: Reading['values']): Reading {
	return {
		id: observedAt,
		deviceId: 'dev',
		labId: 'lab',
		classroomId: 'cls',
		observedAt,
		receivedAt: observedAt,
		values,
		location: { lat: 0, lng: 0 },
		locationSource: 'lab',
		creditedTo: null
	};
}

describe('domain tabs', () => {
	it('shows only the domains a device sends numbers for', () => {
		const tabs = domainTabs(
			[reading('2026-10-20T08:00:00Z', { air_temperature: 27, uv: 3, pressure_trend: 'steady' })],
			parameters
		);
		expect(tabs.map((t) => [t.id, t.parameterIds])).toEqual([
			['AIR', ['air_temperature']],
			['SUN', ['uv']]
		]);
	});

	it('puts values outside the registry under their own tab', () => {
		const tabs = domainTabs([reading('2026-10-20T08:00:00Z', { soil_moisture: 31 })], parameters);
		expect(tabs).toEqual([
			expect.objectContaining({ id: 'OTHER', parameterIds: ['soil_moisture'] })
		]);
	});
});

describe('time ranges', () => {
	it('counts back from the newest reading', () => {
		const readings = [
			reading('2026-10-20T08:00:00Z', {}),
			reading('2026-10-19T09:00:00Z', {}),
			reading('2026-10-15T08:00:00Z', {}),
			reading('2026-09-25T08:00:00Z', {})
		];
		expect(withinRange(readings, '24h').length).toBe(2);
		expect(withinRange(readings, '7d').length).toBe(3);
		expect(withinRange(readings, '30d').length).toBe(4);
		expect(withinRange([], '7d')).toEqual([]);
	});
});
