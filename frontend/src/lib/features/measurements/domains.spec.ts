import { describe, expect, it } from 'vitest';
import { domainsOf } from '#lib/features/measurements/domains.ts';

describe('domains of one device', () => {
	it("keeps only the device's values and drops empty cards", () => {
		const value = (sourceId: string, parameterId: string) => ({
			parameterId,
			value: 1,
			observedAt: '2026-10-20T06:00:00.000Z',
			source: { id: sourceId, name: sourceId, kind: 'device' as const },
			last7Days: []
		});
		const cards = [
			{ domain: null, values: [value('ws', 'air_temperature'), value('soil', 'soil_moisture')] },
			{ domain: null, values: [value('ws', 'wind_avg')] }
		];
		expect(domainsOf(cards, 'soil')).toEqual([
			{ domain: null, values: [value('soil', 'soil_moisture')] }
		]);
		expect(domainsOf(cards, null)).toBe(cards);
	});
});
