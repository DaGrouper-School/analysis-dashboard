import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import { ServiceError } from '../types';
import { createMockServices } from './index';

let clock = Date.parse('2026-10-20T10:00:00Z');
const services = createMockServices({ latency: 0, now: () => clock });

beforeEach(() => {
	resetDb();
	clock = Date.parse('2026-10-20T10:00:00Z');
});

async function signUpWithPhone(phone: string) {
	const challenge = await services.auth.requestPhoneCode(phone);
	return services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
}

describe('phone sign-up (AP-01)', () => {
	it('creates a new user with a verified phone', async () => {
		const { user, isNewUser } = await signUpWithPhone('+251 922 123 456');
		expect(isNewUser).toBe(true);
		expect(user.phone).toBe('+251922123456');
		expect(user.phoneVerified).toBe(true);
		expect(await services.auth.getCurrentUser()).toEqual(user);
	});

	it('signs an existing user back in', async () => {
		const { user, isNewUser } = await signUpWithPhone('+251911000001');
		expect(isNewUser).toBe(false);
		expect(user.id).toBe('usr_abebe');
		expect(user.onboardingCompleted).toBe(true);
	});

	it('rejects invalid numbers', async () => {
		await expect(services.auth.requestPhoneCode('12')).rejects.toMatchObject({
			code: 'invalid_input'
		});
	});

	it('enforces the 30 second resend timer', async () => {
		await services.auth.requestPhoneCode('+251922123456');
		await expect(services.auth.requestPhoneCode('+251922123456')).rejects.toMatchObject({
			code: 'rate_limited'
		});
		clock += 30_000;
		await expect(services.auth.requestPhoneCode('+251922123456')).resolves.toBeTruthy();
	});

	it('rejects a wrong code and locks after five attempts', async () => {
		const challenge = await services.auth.requestPhoneCode('+251922123456');
		const wrong = challenge.devCode === '000000' ? '111111' : '000000';
		for (let i = 0; i < 5; i++) {
			await expect(services.auth.verifyPhoneCode(challenge.requestId, wrong)).rejects.toMatchObject(
				{ code: 'code_invalid' }
			);
		}
		await expect(
			services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!)
		).rejects.toMatchObject({ code: 'rate_limited' });
	});

	it('expires codes after five minutes', async () => {
		const challenge = await services.auth.requestPhoneCode('+251922123456');
		clock += 5 * 60_000;
		await expect(
			services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!)
		).rejects.toMatchObject({ code: 'code_expired' });
	});
});

describe('Google sign-up (AP-01)', () => {
	it('pre-fills name and email for a new account', async () => {
		const { user, isNewUser } = await services.auth.signInWithGoogle('g-hana');
		expect(isNewUser).toBe(true);
		expect(user).toMatchObject({ fullName: 'Hana Bekele', email: 'hana.bekele@example.org' });
		expect(user.phoneVerified).toBe(false);
	});

	it('signs an existing account straight in', async () => {
		const { user, isNewUser } = await services.auth.signInWithGoogle('g-sarah');
		expect(isNewUser).toBe(false);
		expect(user.onboardingCompleted).toBe(true);
	});
});

describe('onboarding (AP-02)', () => {
	it('walks a Google user through phone, profile and address', async () => {
		await services.auth.signInWithGoogle('g-amina');
		await expect(services.profile.completeOnboarding()).rejects.toBeInstanceOf(ServiceError);

		const challenge = await services.profile.startPhoneVerification('+252612345678');
		await services.profile.confirmPhoneVerification(challenge.requestId, challenge.devCode!);
		await services.profile.updateProfile({ fullName: 'Amina Yusuf', email: 'amina@example.org' });
		const [place] = await services.places.autocomplete('Hargeisa');
		await services.profile.updateAddress(await services.places.getAddress(place.placeId));

		const user = await services.profile.completeOnboarding();
		expect(user).toMatchObject({
			onboardingCompleted: true,
			phone: '+252612345678',
			address: { city: 'Hargeisa', country: 'Somalia' }
		});
	});

	it('refuses an email that another account uses', async () => {
		await signUpWithPhone('+251922123456');
		await expect(
			services.profile.updateProfile({ fullName: 'New Person', email: 'sarah.k@example.org' })
		).rejects.toMatchObject({ code: 'conflict', fields: { email: expect.any(String) } });
	});

	it('requires a session', async () => {
		await expect(
			services.profile.updateProfile({ fullName: 'Nobody', email: 'nobody@example.org' })
		).rejects.toMatchObject({ code: 'unauthenticated' });
	});
});

describe('registry (SAM-01, SAM-05)', () => {
	it('registers the 36 station parameters, the manual rain gauge and six device values', async () => {
		const parameters = await services.registry.listParameters();
		expect(parameters.filter((p) => p.source === 'weather_station')).toHaveLength(36);
		expect(parameters).toHaveLength(43);
		expect(new Set(parameters.map((p) => p.domain).filter(Boolean)).size).toBe(6);
		expect(new Set(parameters.map((p) => p.id)).size).toBe(parameters.length);
	});

	it('lists every built-in device type with the values it sends', async () => {
		const types = await services.registry.listDeviceTypes();
		const ids = new Set((await services.registry.listParameters()).map((p) => p.id));
		expect(types.map((t) => t.id)).toContain('soil_sensor');
		for (const type of types) for (const id of type.parameterIds) expect(ids.has(id)).toBe(true);
	});
});

describe('registry changes (SAM-23)', () => {
	const signInAs = (sub: string) => services.auth.signInWithGoogle(sub);
	const soil = { id: 'soil', label: 'Soil', icon: '🌱', description: 'Ground conditions.' };

	it('only lets platform admins change it', async () => {
		await signInAs('g-tesfaye');
		await expect(services.registry.createDomain(soil)).rejects.toMatchObject({ code: 'forbidden' });
		await expect(services.registry.mapParameter('soil_moisture', 'AIR')).rejects.toMatchObject({
			code: 'forbidden'
		});
		await expect(services.registry.listChanges()).rejects.toMatchObject({ code: 'forbidden' });
	});

	it('adds a domain, maps values to it and logs each change', async () => {
		await signInAs('g-leila');
		const domain = await services.registry.createDomain(soil);
		expect(domain.id).toBe('SOIL');
		await expect(services.registry.createDomain(soil)).rejects.toMatchObject({ code: 'conflict' });
		await services.registry.mapParameter('soil_moisture', 'SOIL');
		await services.registry.mapParameter('soil_temperature', 'SOIL');
		const parameters = await services.registry.listParameters();
		expect(parameters.filter((p) => p.domain === 'SOIL').map((p) => p.id)).toEqual([
			'soil_moisture',
			'soil_temperature'
		]);
		const changes = await services.registry.listChanges();
		expect(changes.map((c) => c.summary)).toEqual([
			'Moved Soil temperature (soil_temperature) from no domain to Soil.',
			'Moved Soil moisture (soil_moisture) from no domain to Soil.',
			'Added the Soil domain (SOIL).'
		]);
		expect(changes[0].byName).toBe('Leila M.');
	});

	it('refuses to remove a domain while values are mapped to it', async () => {
		await signInAs('g-leila');
		await expect(services.registry.deleteDomain('SUN')).rejects.toMatchObject({
			code: 'conflict'
		});
		await services.registry.createDomain(soil);
		await services.registry.deleteDomain('SOIL');
		expect((await services.registry.listDomains()).map((d) => d.id)).not.toContain('SOIL');
	});

	it('registers a device type with new and existing values', async () => {
		await signInAs('g-leila');
		const type = await services.registry.createDeviceType({
			label: 'Soil probe v2',
			description: 'Moisture and conductivity.',
			color: '#a77bf3',
			values: [
				{
					id: 'soil_moisture',
					label: 'Soil moisture',
					unit: '%',
					valueType: 'number',
					domain: null
				},
				{
					id: 'soil_ec',
					label: 'Soil conductivity',
					unit: 'mS/cm',
					valueType: 'number',
					domain: 'AIR'
				}
			]
		});
		expect(type).toMatchObject({ id: 'soil_probe_v2', parameterIds: ['soil_moisture', 'soil_ec'] });
		const parameters = await services.registry.listParameters();
		expect(parameters.find((p) => p.id === 'soil_moisture')!.label).toBe('Soil moisture');
		expect(parameters.find((p) => p.id === 'soil_ec')).toMatchObject({
			label: 'Soil conductivity',
			domain: 'AIR',
			source: 'device'
		});
		await expect(
			services.registry.createDeviceType({
				label: 'soil probe V2',
				description: '',
				color: '',
				values: [{ id: 'soil_ec', label: 'EC', unit: '', valueType: 'number', domain: null }]
			})
		).rejects.toMatchObject({ code: 'conflict' });
	});

	it('adds and edits values on an existing type, but not station values', async () => {
		await signInAs('g-leila');
		const type = await services.registry.updateDeviceType('soil_sensor', {
			label: 'Soil sensor',
			description: 'Probe in the ground.',
			color: '#f6921e',
			values: [
				{ id: 'soil_moisture', label: 'Soil water', unit: '%', valueType: 'number', domain: null },
				{
					id: 'soil_temperature',
					label: 'Soil temperature',
					unit: '°C',
					valueType: 'number',
					domain: null
				},
				{ id: 'soil_ph', label: 'Soil pH', unit: '', valueType: 'number', domain: null },
				{
					id: 'air_temperature',
					label: 'Renamed',
					unit: 'K',
					valueType: 'number',
					domain: 'AIR'
				},
				{
					id: 'water_ph',
					label: 'Water pH',
					unit: '',
					valueType: 'number',
					domain: 'WAVES'
				}
			]
		});
		expect(
			(await services.registry.listParameters()).find((p) => p.id === 'water_ph')!.domain
		).toBe('WAVES');
		expect(type.parameterIds.slice(0, 4)).toEqual([
			'soil_moisture',
			'soil_temperature',
			'soil_ph',
			'air_temperature'
		]);
		const parameters = await services.registry.listParameters();
		expect(parameters.find((p) => p.id === 'soil_moisture')!.label).toBe('Soil water');
		expect(parameters.find((p) => p.id === 'soil_ph')).toMatchObject({
			source: 'device',
			domain: null
		});
		expect(parameters.find((p) => p.id === 'air_temperature')).toMatchObject({
			label: 'Air temperature',
			unit: '°C'
		});
		expect((await services.registry.listChanges())[0].summary).toBe(
			'Updated the Soil sensor device type: added soil_ph; edited soil_moisture.'
		);
	});

	it('rejects a device type without valid values', async () => {
		await signInAs('g-leila');
		await expect(
			services.registry.createDeviceType({
				label: 'Probe',
				description: '',
				color: '',
				values: [{ id: 'Bad ID', label: '', unit: '', valueType: 'number', domain: null }]
			})
		).rejects.toMatchObject({
			code: 'invalid_input',
			fields: { 'values.0.id': expect.any(String), 'values.0.label': expect.any(String) }
		});
	});
});
