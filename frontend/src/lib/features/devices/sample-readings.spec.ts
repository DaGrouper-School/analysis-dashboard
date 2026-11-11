import { describe, expect, it } from 'vitest';
import { sampleValues } from '#lib/features/devices/sample-readings.ts';

const parameters = [
	{ id: 'temp', valueType: 'number' as const, min: 0, max: 40 },
	{ id: 'gust', valueType: 'number' as const },
	{ id: 'sky', valueType: 'text' as const, options: ['Clear', 'Cloudy'] }
];

describe('sampleValues', () => {
	it('starts in the middle of the Registry range when the device has no reading', () => {
		expect(sampleValues(['temp'], parameters, {}, () => 0.5)).toEqual({ temp: 20 });
	});

	it('moves on from the last reading, within the range', () => {
		const { temp } = sampleValues(['temp'], parameters, { temp: 39.9 }, () => 1);
		expect(temp).toBe(40);
	});

	it('leaves out a value with neither a last reading nor a range', () => {
		expect(sampleValues(['gust'], parameters, {}, () => 0.5)).toEqual({});
		expect(sampleValues(['gust'], parameters, { gust: 5 }, () => 0.5)).toEqual({ gust: 5 });
	});

	it("picks one of the Registry's answers for a text value", () => {
		expect(sampleValues(['sky'], parameters, {}, () => 0.9)).toEqual({ sky: 'Cloudy' });
	});
});
