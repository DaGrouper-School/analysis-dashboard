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
	identifier: 'soil-ax-01',
	placeName: 'Annex garden',
	country: 'Ethiopia',
	location: { lat: 9.001, lng: 38.701 }
};

describe('labs (AP-05)', () => {
	it('seeds a lab with several device types, each with its own GPS', async () => {
		await signInAs('g-tesfaye');
		const { lab, devices } = await services.labs.get('lab_trinity');
		expect(new Set(devices.map((d) => d.type))).toEqual(
			new Set(['weather_station', 'rain_gauge', 'soil_sensor'])
		);
		const station = devices.find((d) => d.type === 'weather_station')!;
		// The Shashamane station feeds the Trinity lab in Addis Ababa.
		expect(distanceKm(lab.location, station.location)).toBeGreaterThan(150);
		expect(station.samSourceId).toBe('DGC-7K3Q9M:TWS-2041-SH');
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

	it('registers a device as a SAM source under the classroom code', async () => {
		await signInAs('g-tesfaye');
		const device = await services.labs.registerDevice('lab_trinity', deviceInput);
		expect(device).toMatchObject({
			identifier: 'SOIL-AX-01',
			samSourceId: 'DGC-7K3Q9M:SOIL-AX-01',
			location: { lat: 9.001, lng: 38.701 }
		});
		await expect(services.labs.registerDevice('lab_trinity', deviceInput)).rejects.toMatchObject({
			code: 'conflict'
		});
	});

	it('lets a device without its own GPS use the lab location', async () => {
		await signInAs('g-tesfaye');
		const { lab } = await services.labs.get('lab_trinity');
		const device = await services.labs.registerDevice('lab_trinity', {
			...deviceInput,
			location: null
		});
		expect(device).toMatchObject({ ownLocation: false, location: lab.location });
	});

	it('rejects invalid GPS coordinates', async () => {
		await signInAs('g-tesfaye');
		await expect(
			services.labs.registerDevice('lab_trinity', { ...deviceInput, location: { lat: 95, lng: 0 } })
		).rejects.toMatchObject({ code: 'invalid_input', fields: { location: expect.any(String) } });
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
	});

	it('hides labs from people outside the classroom', async () => {
		await signInAs('g-sarah');
		await expect(services.labs.get('lab_trinity')).rejects.toMatchObject({ code: 'not_found' });
		expect(await services.labs.listMyDevices()).toEqual([]);
	});

	it('removes a device and frees its identifier', async () => {
		await signInAs('g-tesfaye');
		const device = await services.labs.registerDevice('lab_trinity', deviceInput);
		await services.labs.removeDevice(device.id);
		await expect(services.labs.registerDevice('lab_trinity', deviceInput)).resolves.toBeTruthy();
	});

	it('lists the devices of my labs, and every device publicly without SAM sources', async () => {
		await signInAbebe();
		const mine = await services.labs.listMyDevices();
		expect(mine.map((d) => d.labId)).toEqual(['lab_trinity', 'lab_trinity', 'lab_trinity']);
		const all = await services.registry.listDevices();
		expect(all.length).toBe(5);
		expect(all.every((d) => d.samSourceId === null)).toBe(true);
	});
});
