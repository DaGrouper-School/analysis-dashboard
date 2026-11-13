import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import type { InstitutionAnswers } from '#lib/domain/types.ts';
import { createMockServices } from './index';

const services = createMockServices({ latency: 0 });
const signInAs = (sub: string) => services.auth.signInWithGoogle(sub);
const IMAGE = 'data:image/jpeg;base64,/9j/4AAQSkZJRg==';

const ANSWERS: InstitutionAnswers = {
	photo: IMAGE,
	photoAlt: 'The school on the harbour',
	foundedYear: '1962',
	history: 'A harbour school whose students track storms on the shoreline.',
	ecosystems: ['Shoreline', 'Seagrass'],
	hub: 'Earth Care',
	website: 'https://bay-street.example',
	showPublicly: 'yes'
};

beforeEach(() => resetDb());

/** A platform admin sets up a classroom; Sarah joins it as its institution head. */
async function newClassroomWithHead() {
	await signInAs('g-leila');
	const created = await services.classrooms.create({
		name: 'Harbour Classroom',
		institutionName: 'Bay Street School',
		headEmail: 'sarah.k@example.org'
	});
	const { invitations } = await services.classrooms.get(created.id);
	await signInAs('g-sarah');
	await services.classrooms.acceptInvitation(invitations[0].token);
	return created.id;
}

describe('institution setup (AP-40)', () => {
	it('is waiting for the head of a new classroom', async () => {
		const id = await newClassroomWithHead();
		const setup = await services.classrooms.institutionSetup(id);
		expect(setup).toMatchObject({
			step: 0,
			completedAt: null,
			institutionName: 'Bay Street School'
		});
		expect(setup.answers).toMatchObject({ photo: null, history: '', showPublicly: '' });
		const detail = await services.classrooms.get(id);
		expect(detail.permissions.setUpInstitution).toBe(true);
		expect(detail.setup).toEqual({ institutionDone: false, hostAgreementSigned: false });
	});

	it('comes from configuration', async () => {
		const form = await services.classrooms.institutionSetupForm();
		expect(form.steps.length).toBeGreaterThan(0);
		const steps = new Set(form.steps.map((s) => s.id));
		expect(form.questions.every((q) => steps.has(q.step))).toBe(true);
		expect(form.questions.map((q) => q.field)).toContain('photo');
	});

	it('saves unfinished answers and the step, for "Finish later"', async () => {
		const id = await newClassroomWithHead();
		const draft = { ...ANSWERS, photo: null, photoAlt: '', history: '', showPublicly: '' as const };
		const saved = await services.classrooms.saveInstitutionSetup(id, {
			answers: draft,
			step: 2,
			finish: false
		});
		expect(saved).toMatchObject({ step: 2, completedAt: null });
		expect((await services.classrooms.institutionSetup(id)).answers.hub).toBe('Earth Care');
	});

	it('still refuses answers that are wrong, finished or not', async () => {
		const id = await newClassroomWithHead();
		await expect(
			services.classrooms.saveInstitutionSetup(id, {
				answers: { ...ANSWERS, foundedYear: 'long ago', website: 'bay street' },
				step: 1,
				finish: false
			})
		).rejects.toMatchObject({
			code: 'invalid_input',
			fields: { foundedYear: expect.any(String), website: expect.any(String) }
		});
	});

	it('needs every required answer to finish', async () => {
		const id = await newClassroomWithHead();
		await expect(
			services.classrooms.saveInstitutionSetup(id, {
				answers: { ...ANSWERS, photo: null, history: '  ' },
				step: 4,
				finish: true
			})
		).rejects.toMatchObject({ fields: { photo: expect.any(String), history: expect.any(String) } });
	});

	it('fills in the institution overview when finished', async () => {
		const id = await newClassroomWithHead();
		const done = await services.classrooms.saveInstitutionSetup(id, {
			answers: ANSWERS,
			step: 4,
			finish: true
		});
		expect(done.completedAt).not.toBeNull();
		expect((await services.classrooms.get(id)).setup?.institutionDone).toBe(true);
		// Shown once a platform admin has the signed host agreement.
		await signInAs('g-leila');
		await services.classrooms.setHostAgreement(id, true);
		await services.auth.signOut();
		const station = await services.publicSite.station(id);
		expect(station.institution).toMatchObject({
			photoUrl: IMAGE,
			photoAlt: 'The school on the harbour',
			foundedYear: 1962,
			hub: 'Earth Care',
			website: 'https://bay-street.example',
			showPublicly: true
		});
	});

	it('takes the ecosystems from the host application', async () => {
		await signInAs('g-leila');
		await services.hosting.decide('happ_andros', true);
		const created = await services.classrooms.create({
			name: 'Andros Classroom',
			institutionName: 'North Andros High School',
			headEmail: 'sarah.k@example.org',
			applicationId: 'happ_andros'
		});
		const { invitations } = await services.classrooms.get(created.id);
		await signInAs('g-sarah');
		await services.classrooms.acceptInvitation(invitations[0].token);
		const setup = await services.classrooms.institutionSetup(created.id);
		expect(setup.answers.ecosystems).toEqual(['Mangrove', 'Coral reef']);
	});

	it('is only for the institution head', async () => {
		await signInAs('g-marcus');
		await expect(services.classrooms.institutionSetup('cls_trinity')).rejects.toMatchObject({
			code: 'forbidden'
		});
		await signInAs('g-leila');
		await expect(
			services.classrooms.saveInstitutionSetup('cls_trinity', {
				answers: ANSWERS,
				step: 0,
				finish: false
			})
		).rejects.toMatchObject({ code: 'forbidden' });
	});
});

describe('showing the institution publicly', () => {
	it('hides it from visitors when the head says no, but not from members', async () => {
		await signInAs('g-tesfaye');
		const { answers } = await services.classrooms.institutionSetup('cls_trinity');
		await services.classrooms.saveInstitutionSetup('cls_trinity', {
			answers: { ...answers, photo: IMAGE, showPublicly: 'no' },
			step: 4,
			finish: true
		});
		expect((await services.publicSite.station('cls_trinity')).institution).not.toBeNull();
		await services.auth.signOut();
		expect((await services.publicSite.station('cls_trinity')).institution).toBeNull();
	});
});

describe('host agreement', () => {
	it('is marked signed by a platform admin', async () => {
		const id = await newClassroomWithHead();
		await signInAs('g-leila');
		expect(await services.classrooms.setHostAgreement(id, true)).toMatchObject({
			hostAgreementSigned: true
		});
		expect((await services.classrooms.get(id)).setup?.hostAgreementSigned).toBe(true);
	});

	it('is never marked by the institution head', async () => {
		const id = await newClassroomWithHead();
		await expect(services.classrooms.setHostAgreement(id, true)).rejects.toMatchObject({
			code: 'forbidden'
		});
	});
});
