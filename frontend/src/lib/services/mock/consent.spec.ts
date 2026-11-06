import { beforeEach, describe, expect, it } from 'vitest';
import { readDb, resetDb } from '#lib/data/mock-db.ts';
import type { ObservationInput } from '#lib/domain/types.ts';
import { createMockServices } from './index';

const services = createMockServices({ latency: 0 });
const TOKEN = 'consent_liya_demo';

beforeEach(() => resetDb());

async function signIn(phone: string) {
	const challenge = await services.auth.requestPhoneCode(phone);
	await services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
}
const signInLiya = () => signIn('+251911000003');

const sighting = (location: ObservationInput['location'] = null): ObservationInput => ({
	templateId: 'tpl_species_sighting',
	labId: 'lab_trinity',
	deviceId: null,
	observedAt: '2026-10-20T07:00:00.000Z',
	values: { species_name: 'Hooded vulture', species_count: 3 },
	location
});

describe('guardian consent for minors (AP-04)', () => {
	it('refuses a minor’s observations until a guardian gives consent', async () => {
		await signInLiya();
		await expect(services.labs.submitObservation(sighting())).rejects.toMatchObject({
			code: 'forbidden',
			message: expect.stringContaining('guardian needs to give consent')
		});
		// Offline records are refused one by one, with the same reason.
		const synced = await services.labs.syncObservations({
			sentAt: new Date().toISOString(),
			records: [{ clientId: 'c1', observation: sighting(), credit: false }]
		});
		expect(synced.results[0]).toMatchObject({
			outcome: 'rejected',
			error: expect.stringContaining('consent')
		});
		await expect(
			services.labs.uploadReadings('dev_shashamane_gauge', [
				{
					observedAt: '2026-10-20T07:00:00.000Z',
					values: { rain_gauge_daily_accumulation: 2 },
					location: null
				}
			])
		).rejects.toMatchObject({ code: 'forbidden' });
	});

	it('shows the guardian only the first name, and records their answer for SAM', async () => {
		const request = await services.consent.get(TOKEN);
		expect(request).toMatchObject({
			contributorName: 'Liya',
			guardianName: 'Almaz A.',
			status: 'pending',
			coordinateDecimals: 2
		});
		expect(request.classroomNames).toHaveLength(1);
		const given = await services.consent.decide(TOKEN, true);
		expect(given.status).toBe('given');
		expect(readDb().samAssertions).toEqual([
			expect.objectContaining({ userId: 'usr_liya', attribute: 'consent', value: 'given' })
		]);
	});

	it('with consent, stores the minor’s observations without their name or precise position', async () => {
		await services.consent.decide(TOKEN, true);
		await signInLiya();
		const stored = await services.labs.submitObservation(
			sighting({ lat: 9.04567, lng: 38.76123 }),
			{
				credit: true
			}
		);
		expect(stored).toMatchObject({
			creditedTo: null,
			locationSource: 'device',
			location: { lat: 9.05, lng: 38.76 }
		});
	});

	it('stops new observations once consent is withdrawn', async () => {
		await services.consent.decide(TOKEN, true);
		await services.consent.decide(TOKEN, false);
		await signInLiya();
		expect((await services.auth.getCurrentUser())?.consent).toBe('withdrawn');
		await expect(services.labs.submitObservation(sighting())).rejects.toMatchObject({
			code: 'forbidden',
			message: expect.stringContaining('withdrew')
		});
		expect(readDb().samAssertions.map((a) => a.value)).toEqual(['given', 'withdrawn']);
	});

	it('lets a contributor say they are under 18, and replaces the old link on a new request', async () => {
		await signIn('+251911000001');
		await expect(
			services.profile.requestGuardianConsent({ guardianName: '', guardianContact: 'nope' })
		).rejects.toMatchObject({
			code: 'invalid_input',
			fields: { guardianName: expect.any(String), guardianContact: expect.any(String) }
		});
		const first = await services.profile.requestGuardianConsent({
			guardianName: 'Tigist T.',
			guardianContact: 'Tigist@Example.org'
		});
		expect(first).toMatchObject({ status: 'pending', guardianContact: 'tigist@example.org' });
		expect(await services.auth.getCurrentUser()).toMatchObject({ minor: true, consent: 'pending' });
		const second = await services.profile.requestGuardianConsent({
			guardianName: 'Tigist T.',
			guardianContact: '+251 911 000 009'
		});
		expect(second.guardianContact).toBe('+251911000009');
		await expect(services.consent.get(first.devToken!)).rejects.toMatchObject({
			code: 'not_found'
		});
		expect(await services.profile.getGuardianConsent()).toMatchObject({
			devToken: second.devToken
		});
	});

	it('leaves adults alone', async () => {
		await signIn('+251911000001');
		expect(await services.profile.getGuardianConsent()).toBeNull();
		const stored = await services.labs.submitObservation(sighting({ lat: 9.04567, lng: 38.76123 }));
		expect(stored.location).toEqual({ lat: 9.04567, lng: 38.76123 });
	});
});
