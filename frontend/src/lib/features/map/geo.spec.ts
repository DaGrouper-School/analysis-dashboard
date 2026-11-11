import { describe, expect, it } from 'vitest';
import { formatCoordinates } from '#lib/features/map/geo.ts';

describe('formatCoordinates', () => {
	it('rounds a position to two decimals with its hemispheres', () => {
		expect(formatCoordinates({ lat: 9.0301, lng: 38.7401 })).toBe('9.03° N, 38.74° E');
		expect(formatCoordinates({ lat: 25.08, lng: -77.34 })).toBe('25.08° N, 77.34° W');
	});
});
