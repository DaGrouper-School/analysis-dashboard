import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import type { ObservationInput, ObservationTemplate } from '#lib/domain/types.ts';
import { createMockServices } from '#lib/services/mock/index.ts';
import { readEntries, withDeviceTemplates } from './observations';

const services = createMockServices({ latency: 0 });

beforeEach(() => resetDb());

async function signInAbebe() {
	const challenge = await services.auth.requestPhoneCode('+251911000001');
	await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
}

async function registry() {
	await signInAbebe();
	const [templates, parameters] = await Promise.all([
		services.registry.listObservationTemplates(),
		services.registry.listParameters()
	]);
	const template = (id: string) => templates.find((t) => t.id === id)!;
	return { templates, parameters, template };
}

const rainInput = (values: ObservationInput['values']): ObservationInput => ({
	templateId: 'tpl_rain_gauge',
	labId: 'lab_trinity',
	deviceId: 'dev_shashamane_gauge',
	observedAt: '2026-10-20T07:00:00.000Z',
	values,
	location: null
});

describe('observation templates (AP-16)', () => {
	it('lists a template for each domain example, and one for every device type', async () => {
		const { templates } = await registry();
		expect(templates.map((t) => t.id)).toEqual(
			expect.arrayContaining(['tpl_rain_gauge', 'tpl_coral_spawning', 'tpl_species_sighting'])
		);
		// "Other" has no values, so there is nothing to fill in for it.
		const covered = new Set(templates.map((t) => t.deviceType));
		expect([...covered].sort()).toEqual(
			[
				null,
				'air_quality_sensor',
				'rain_gauge',
				'soil_sensor',
				'water_probe',
				'weather_station'
			].sort()
		);
	});

	it('gives a device type a platform admin registers a plain form', () => {
		const derived = withDeviceTemplates(
			[],
			[
				{
					id: 'h2s',
					label: 'Hydrogen sulfide',
					description: 'Gas sensor.',
					color: '#d4b72c',
					icon: 'flask',
					parameterIds: ['h2s', 'pm10']
				}
			],
			[]
		);
		expect(derived).toEqual<ObservationTemplate[]>([
			{
				id: 'device:h2s',
				label: 'Hydrogen sulfide reading',
				description: 'Gas sensor.',
				domain: null,
				deviceType: 'h2s',
				fields: [
					{ parameterId: 'h2s', required: true },
					{ parameterId: 'pm10', required: false }
				]
			}
		]);
	});
});

describe('pre-submission checks', () => {
	it('catches missing fields, non-numbers and out-of-range values', async () => {
		const { parameters, template } = await registry();
		const { errors } = readEntries(template('tpl_water_check'), parameters, {
			water_temperature: { value: 'warm', unit: '°C' },
			water_ph: { value: '15', unit: '' }
		});
		expect(errors).toEqual({
			water_temperature: 'Water temperature must be a number.',
			water_ph: 'Water pH should be 0 to 14.'
		});
		const missing = readEntries(template('tpl_species_sighting'), parameters, {});
		expect(missing.errors).toEqual({
			species_name: 'Species is required.',
			species_count: 'How many is required.'
		});
	});

	it('converts other units, and points out a unit that was left wrong', async () => {
		const { parameters, template } = await registry();
		const inches = readEntries(template('tpl_rain_gauge'), parameters, {
			rain_gauge_daily_accumulation: { value: '0.5', unit: 'in' }
		});
		expect(inches).toEqual({ values: { rain_gauge_daily_accumulation: 12.7 }, errors: {} });
		// 86 °F typed with °C still selected.
		const { errors } = readEntries(template('tpl_station_spot_check'), parameters, {
			air_temperature: { value: '86', unit: '°C' }
		});
		expect(errors.air_temperature).toBe(
			'Air temperature should be -60 to 60 °C. If you measured in °F, switch the unit.'
		);
		const fahrenheit = readEntries(template('tpl_station_spot_check'), parameters, {
			air_temperature: { value: '86', unit: '°F' }
		});
		expect(fahrenheit.values).toEqual({ air_temperature: 30 });
	});

	it('only accepts the listed answers', async () => {
		const { parameters, template } = await registry();
		const { errors } = readEntries(template('tpl_coral_spawning'), parameters, {
			coral_spawning_observed: { value: 'Maybe', unit: '' }
		});
		expect(errors.coral_spawning_observed).toBe('Pick one of: Yes, No, Not sure.');
	});
});

describe('submitting observations', () => {
	it('records a device reading tagged with the classroom, on the lab location without GPS', async () => {
		await signInAbebe();
		const stored = await services.labs.submitObservation(
			rainInput({ rain_gauge_daily_accumulation: 6.5 })
		);
		expect(stored).toMatchObject({
			templateId: 'tpl_rain_gauge',
			deviceId: 'dev_shashamane_gauge',
			classroomId: 'cls_trinity',
			locationSource: 'lab',
			location: { lat: 9.03, lng: 38.74 },
			creditedTo: null
		});
		// It is one of the gauge's readings now, so it shows on its charts.
		const readings = await services.labs.deviceReadings('dev_shashamane_gauge');
		expect(readings.map((r) => r.id)).toContain(stored.id);
	});

	it('records a field observation with GPS and credit when asked', async () => {
		await signInAbebe();
		const stored = await services.labs.submitObservation(
			{
				templateId: 'tpl_species_sighting',
				labId: 'lab_trinity',
				deviceId: null,
				observedAt: '2026-10-20T07:00:00.000Z',
				values: { species_name: 'Hooded vulture', species_count: 3 },
				location: { lat: 9.05, lng: 38.76 }
			},
			{ credit: true }
		);
		expect(stored).toMatchObject({
			deviceId: null,
			classroomId: 'cls_trinity',
			locationSource: 'device',
			creditedTo: 'Abebe T.'
		});
		const detail = await services.labs.get('lab_trinity');
		expect(detail.observations.map((o) => o.id)).toEqual([stored.id]);
	});

	it('runs the checks again, and never takes a device that is not registered', async () => {
		await signInAbebe();
		await expect(
			services.labs.submitObservation(rainInput({ rain_gauge_daily_accumulation: 900 }))
		).rejects.toMatchObject({
			code: 'invalid_input',
			fields: { rain_gauge_daily_accumulation: 'Manual rain gauge, daily should be 0 to 500 mm.' }
		});
		await expect(
			services.labs.submitObservation({
				...rainInput({ rain_gauge_daily_accumulation: 4 }),
				deviceId: 'dev_new_gauge'
			})
		).rejects.toMatchObject({ code: 'invalid_input', fields: { deviceId: expect.any(String) } });
		// A device of another type does not fit the rain gauge form either.
		await expect(
			services.labs.submitObservation({
				...rainInput({ rain_gauge_daily_accumulation: 4 }),
				deviceId: 'dev_trinity_soil'
			})
		).rejects.toMatchObject({ code: 'invalid_input' });
	});

	it('lets researchers read but not record', async () => {
		await services.auth.signInWithGoogle('g-marcus');
		expect(await services.labs.observationTargets()).toEqual([]);
		await expect(
			services.labs.submitObservation(rainInput({ rain_gauge_daily_accumulation: 4 }))
		).rejects.toMatchObject({ code: 'forbidden' });
	});

	it('offers community scientists their labs and registered devices', async () => {
		await signInAbebe();
		const targets = await services.labs.observationTargets();
		expect(targets.map((t) => t.lab.id)).toEqual(['lab_trinity']);
		expect(targets[0].devices.map((d) => d.id)).toContain('dev_shashamane_gauge');
	});
});
