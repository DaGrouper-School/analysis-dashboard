import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import type { CreateLabInput, RegisterDeviceInput } from '#lib/domain/types.ts';
import { distanceKm } from '#lib/domain/validation.ts';
import { createMockServices } from './index';

const services = createMockServices({ latency: 0 });

beforeEach(() => resetDb());

const signInAs = (sub: string) => services.auth.signInWithGoogle(sub);

async function signInAbebe() {
	const challenge = await services.auth.requestPhoneCode('+251911000001');
	await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
}

const labInput: CreateLabInput = {
	name: 'Second Lab',
	placeName: 'Trinity School annex',
	country: 'Ethiopia',
	location: { lat: 9.0, lng: 38.7 },
	siteProfile: {
		setting: 'urban',
		description: 'An annex lab near the main school.',
		researchQuestions: ['  How dusty is the air in March?  ', '']
	}
};

const deviceInput: RegisterDeviceInput = {
	name: 'Annex Soil Sensor',
	type: 'soil_sensor',
	identifier: 'soil-ax-01'
};

const reading = (location: { lat: number; lng: number } | null) => ({
	observedAt: '2026-10-20T10:00:00.000Z',
	values: { soil_moisture: 28 },
	location
});

describe('labs (AP-05)', () => {
	it('seeds a lab with several device types; GPS comes with readings, not devices', async () => {
		await signInAs('g-tesfaye');
		const { lab, devices, readings } = await services.labs.get('lab_trinity');
		expect(new Set(devices.map((d) => d.type))).toEqual(
			new Set(['weather_station', 'rain_gauge', 'soil_sensor'])
		);
		const station = devices.find((d) => d.type === 'weather_station')!;
		expect(station).not.toHaveProperty('location');
		expect(station.samSourceId).toBe('DGC-7K3Q9M:TWS-2041-SH');
		// The Shashamane station feeds the Trinity lab in Addis Ababa, and has moved between readings.
		const stationReadings = readings.filter((r) => r.deviceId === station.id);
		expect(stationReadings.length).toBe(2);
		expect(stationReadings.every((r) => r.locationSource === 'device')).toBe(true);
		expect(distanceKm(lab.location, stationReadings[0].location)).toBeGreaterThan(150);
		expect(stationReadings[0].location).not.toEqual(stationReadings[1].location);
		// The rain gauge has no GPS: its reading stands in with the lab location, flagged.
		const gauge = readings.find((r) => r.deviceId === 'dev_shashamane_gauge')!;
		expect(gauge).toMatchObject({ locationSource: 'lab', location: lab.location });
	});

	it('lets a focal person add a lab with a site profile', async () => {
		await signInAs('g-tesfaye');
		const lab = await services.labs.create('cls_trinity', labInput);
		expect(lab.siteProfile.researchQuestions).toEqual(['How dusty is the air in March?']);
		expect((await services.labs.listForClassroom('cls_trinity')).map((l) => l.id)).toContain(
			lab.id
		);
	});

	it('requires the site profile questionnaire', async () => {
		await signInAs('g-tesfaye');
		await expect(
			services.labs.create('cls_trinity', {
				...labInput,
				siteProfile: { setting: 'urban', description: '', researchQuestions: [] }
			})
		).rejects.toMatchObject({
			code: 'invalid_input',
			fields: { description: expect.any(String), researchQuestions: expect.any(String) }
		});
	});

	it('registers a device as a SAM source under the classroom code, with no location', async () => {
		await signInAs('g-tesfaye');
		const device = await services.labs.registerDevice('lab_trinity', deviceInput);
		expect(device).toMatchObject({
			identifier: 'SOIL-AX-01',
			samSourceId: 'DGC-7K3Q9M:SOIL-AX-01'
		});
		expect(device).not.toHaveProperty('location');
		await expect(services.labs.registerDevice('lab_trinity', deviceInput)).rejects.toMatchObject({
			code: 'conflict'
		});
	});

	it('tags uploaded readings with the classroom and keeps GPS per reading', async () => {
		await signInAs('g-tesfaye');
		const device = await services.labs.registerDevice('lab_trinity', deviceInput);
		const [first, second] = await services.labs.uploadReadings(device.id, [
			reading({ lat: 9.001, lng: 38.701 }),
			reading({ lat: 9.2, lng: 38.9 })
		]);
		expect(first).toMatchObject({
			classroomId: 'cls_trinity',
			labId: 'lab_trinity',
			locationSource: 'device',
			location: { lat: 9.001, lng: 38.701 }
		});
		expect(second.location).toEqual({ lat: 9.2, lng: 38.9 });
		const { readings } = await services.labs.get('lab_trinity');
		expect(readings.filter((r) => r.deviceId === device.id).length).toBe(2);
	});

	it('falls back to the lab location when a reading has no GPS, and flags it', async () => {
		await signInAs('g-tesfaye');
		const { lab } = await services.labs.get('lab_trinity');
		const device = await services.labs.registerDevice('lab_trinity', deviceInput);
		const [stored] = await services.labs.uploadReadings(device.id, [reading(null)]);
		expect(stored).toMatchObject({ locationSource: 'lab', location: lab.location });
	});

	it('rejects readings with invalid GPS coordinates', async () => {
		await signInAs('g-tesfaye');
		await expect(
			services.labs.uploadReadings('dev_trinity_soil', [reading({ lat: 95, lng: 0 })])
		).rejects.toMatchObject({ code: 'invalid_input', fields: { location: expect.any(String) } });
	});

	it('lets focal people edit the lab', async () => {
		await signInAs('g-tesfaye');
		const lab = await services.labs.update('lab_trinity', {
			...labInput,
			name: 'Trinity Field Lab'
		});
		expect(lab).toMatchObject({ id: 'lab_trinity', name: 'Trinity Field Lab' });
		await signInAbebe();
		await expect(services.labs.update('lab_trinity', labInput)).rejects.toMatchObject({
			code: 'forbidden'
		});
	});

	it('lets community scientists see devices but not register them or see SAM sources', async () => {
		await signInAbebe();
		const detail = await services.labs.get('lab_trinity');
		expect(detail.canManage).toBe(false);
		expect(detail.devices.length).toBe(3);
		expect(detail.devices.every((d) => d.samSourceId === null)).toBe(true);
		await expect(services.labs.registerDevice('lab_trinity', deviceInput)).rejects.toMatchObject({
			code: 'forbidden'
		});
		await expect(services.labs.create('cls_trinity', labInput)).rejects.toMatchObject({
			code: 'forbidden'
		});
		await expect(
			services.labs.uploadReadings('dev_trinity_soil', [reading(null)])
		).rejects.toMatchObject({ code: 'forbidden' });
	});

	it('hides labs from people outside the classroom', async () => {
		await signInAs('g-sarah');
		await expect(services.labs.get('lab_trinity')).rejects.toMatchObject({ code: 'not_found' });
		expect(await services.labs.currentConditions()).toEqual([]);
	});

	it('removes a device and frees its identifier', async () => {
		await signInAs('g-tesfaye');
		const device = await services.labs.registerDevice('lab_trinity', deviceInput);
		await services.labs.removeDevice(device.id);
		await expect(services.labs.registerDevice('lab_trinity', deviceInput)).resolves.toBeTruthy();
	});

	it('shows current conditions only from classrooms I belong to', async () => {
		await signInAbebe();
		const conditions = await services.labs.currentConditions();
		expect(new Set(conditions.map((c) => c.device.labId))).toEqual(new Set(['lab_trinity']));
		expect(conditions.length).toBe(3);
		expect(conditions.every((c) => c.classroomName === 'Trinity Lab Classroom')).toBe(true);
		// Community scientists never see the classroom code, even inside a SAM source ID.
		expect(conditions.every((c) => c.device.samSourceId === null)).toBe(true);
		// A platform admin who is not a member sees no conditions either.
		await signInAs('g-marcus');
		expect(await services.labs.currentConditions()).toEqual([]);
	});

	it('attaches active alerts to current conditions', async () => {
		await signInAs('g-tesfaye');
		const conditions = await services.labs.currentConditions();
		const station = conditions.find((c) => c.device.id === 'dev_shashamane_ws')!;
		expect(station.alerts.map((a) => a.title)).toEqual(['High UV']);
		expect(conditions.find((c) => c.device.id === 'dev_trinity_soil')!.alerts).toEqual([]);
	});
});
