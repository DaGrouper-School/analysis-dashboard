import { describe, expect, it } from 'vitest';
import type { Device, ObservationTemplate } from '#lib/domain/types.ts';
import {
	deviceProblem,
	localNow,
	observeSteps,
	templateGroups,
	timeProblem
} from './observe-form.ts';

const template = (id: string, domain: string | null) =>
	({ id, label: id, description: '', domain, deviceType: null, fields: [] }) as ObservationTemplate;

describe('observeSteps', () => {
	it('asks for the device only when the template is read from one', () => {
		expect(observeSteps(false, true)).toEqual(['what', 'device', 'values', 'review']);
		expect(observeSteps(false, false)).toEqual(['what', 'values', 'review']);
	});

	it('skips picking when opened for one sensor', () => {
		expect(observeSteps(true, true)).toEqual(['values', 'review']);
	});
});

describe('templateGroups', () => {
	it("follows the registry's domain order and puts unknown domains under Other", () => {
		const domains = [
			{ id: 'RAIN', label: 'Rain', icon: 'rain', description: '' },
			{ id: 'AIR', label: 'Air', icon: 'air', description: '' }
		];
		const groups = templateGroups(domains, [
			template('a', 'AIR'),
			template('b', null),
			template('c', 'RAIN'),
			template('d', 'GONE')
		]);
		expect(groups.map((g) => [g.label, g.templates.map((t) => t.id)])).toEqual([
			['Rain', ['c']],
			['Air', ['a']],
			['Other', ['b', 'd']]
		]);
	});
});

describe('localNow', () => {
	it('is minutes precision, ready for a datetime-local field', () => {
		expect(localNow()).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/);
	});
});

describe('deviceProblem', () => {
	const read = { ...template('t', null), deviceType: 'soil_sensor' } as ObservationTemplate;
	const probe = { id: 'd1', type: 'soil_sensor' } as Device;
	const types = [{ id: 'soil_sensor', label: 'Soil sensor' }] as never;

	it('needs nothing for a field observation or a picked device', () => {
		expect(deviceProblem(template('t', null), [], null, types)).toBe('');
		expect(deviceProblem(read, [probe], probe, types)).toBe('');
	});

	it('asks for a pick, or says none is registered', () => {
		expect(deviceProblem(read, [probe, probe], null, types)).toBe(
			'Choose the device you read this from.'
		);
		expect(deviceProblem(read, [], null, types)).toBe('No soil sensor is registered to this lab.');
	});
});

describe('timeProblem', () => {
	const now = Date.parse('2026-11-11T12:00:00');
	it('needs a time that is not in the future', () => {
		expect(timeProblem('', now)).toBe('Enter when you observed it.');
		expect(timeProblem('2026-11-11T12:04', now)).toBe('');
		expect(timeProblem('2026-11-11T12:10', now)).toBe('That time is in the future.');
	});
});
