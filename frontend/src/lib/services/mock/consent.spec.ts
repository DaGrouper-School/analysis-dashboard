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
			recordedByParticipantId: 100006,
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

describe('paper consent recorded by the classroom (AP-38)', () => {
	const signInHead = () => services.auth.signInWithGoogle('g-tesfaye');
	const today = new Date().toISOString().slice(0, 10);

	it('shows the head every minor in the classroom with where consent stands', async () => {
		await signInHead();
		const detail = await services.classrooms.get('cls_trinity');
		expect(detail.permissions.manageConsent).toBe(true);
		expect(await services.classrooms.listConsent('cls_trinity')).toEqual([
			expect.objectContaining({
				userId: 'usr_liya',
				status: 'pending',
				method: 'link',
				guardianName: 'Almaz A.'
			})
		]);
	});

	it('unblocks the minor, asserts it to SAM, and shows the minor only their status', async () => {
		await signInHead();
		const recorded = await services.classrooms.recordPaperConsent('cls_trinity', 'usr_liya', {
			give: true,
			guardianName: ' Almaz A. ',
			signedOn: '2026-11-01'
		});
		expect(recorded).toMatchObject({
			status: 'given',
			method: 'paper',
			guardianName: 'Almaz A.',
			signedOn: '2026-11-01',
			recordedByName: 'Tesfaye G.'
		});
		expect(readDb().samAssertions).toEqual([
			expect.objectContaining({ userId: 'usr_liya', attribute: 'consent', value: 'given' })
		]);

		await signInLiya();
		expect(await services.profile.getGuardianConsent()).toMatchObject({
			status: 'given',
			method: 'paper',
			signedOn: '2026-11-01',
			recordedByName: 'Tesfaye G.'
		});
		await expect(services.labs.submitObservation(sighting())).resolves.toBeTruthy();
		await expect(services.classrooms.listConsent('cls_trinity')).rejects.toMatchObject({
			code: 'forbidden'
		});
	});

	it('records a withdrawal the same way, and the guardian link still works', async () => {
		await signInHead();
		await services.classrooms.recordPaperConsent('cls_trinity', 'usr_liya', {
			give: true,
			guardianName: 'Almaz A.',
			signedOn: today
		});
		await services.classrooms.recordPaperConsent('cls_trinity', 'usr_liya', {
			give: false,
			guardianName: 'Almaz A.',
			signedOn: today
		});
		await signInLiya();
		await expect(services.labs.submitObservation(sighting())).rejects.toMatchObject({
			code: 'forbidden',
			message: expect.stringContaining('withdrew')
		});
		expect((await services.consent.decide(TOKEN, true)).status).toBe('given');
		expect(readDb().samAssertions.map((a) => a.value)).toEqual(['given', 'withdrawn', 'given']);
	});

	it('checks the sheet’s details, and refuses everyone but the head and facilitators', async () => {
		await signInHead();
		await expect(
			services.classrooms.recordPaperConsent('cls_trinity', 'usr_liya', {
				give: true,
				guardianName: '',
				signedOn: '2999-01-01'
			})
		).rejects.toMatchObject({
			code: 'invalid_input',
			fields: { guardianName: expect.any(String), signedOn: expect.any(String) }
		});
		// Only members under 18 have consent to record.
		await expect(
			services.classrooms.recordPaperConsent('cls_trinity', 'usr_abebe', {
				give: true,
				guardianName: 'Someone',
				signedOn: today
			})
		).rejects.toMatchObject({ code: 'not_found' });

		// A platform admin is not at the classroom: no consent tab, no sheet, no recording.
		await services.auth.signInWithGoogle('g-leila');
		expect((await services.classrooms.get('cls_trinity')).permissions.manageConsent).toBe(false);
		await expect(services.classrooms.consentSheet('cls_trinity')).rejects.toMatchObject({
			code: 'forbidden'
		});
		await expect(
			services.classrooms.recordPaperConsent('cls_trinity', 'usr_liya', {
				give: true,
				guardianName: 'Almaz A.',
				signedOn: today
			})
		).rejects.toMatchObject({ code: 'forbidden' });
		expect(readDb().samAssertions).toEqual([]);
	});

	it('fills the printable sheet with the classroom’s details from content', async () => {
		await signInHead();
		const sheet = await services.classrooms.consentSheet('cls_trinity');
		const state = readDb();
		expect(sheet).toMatchObject({
			classroomName: 'Trinity Lab Classroom',
			institutionName: 'Trinity School, Addis Ababa',
			minors: [{ userId: 'usr_liya', fullName: 'Liya A.' }]
		});
		expect(sheet.content.fields).toEqual(state.siteContent.consentSheet.fields);
		const text = JSON.stringify(sheet.content);
		expect(text).not.toMatch(/\{(classroom|institution|decimals)\}/);
		expect(sheet.content.intro).toContain('Trinity School, Addis Ababa');
		expect(text).toContain(`${state.privacy.minorCoordinateDecimals} decimal places`);
	});

	it('lets a minor say they are under 18 without sending a link', async () => {
		await signIn('+251911000001');
		const user = await services.profile.markMinor();
		expect(user).toMatchObject({ minor: true, consent: 'pending' });
		expect(await services.profile.getGuardianConsent()).toBeNull();
		await expect(services.labs.submitObservation(sighting())).rejects.toMatchObject({
			code: 'forbidden'
		});
	});
});
