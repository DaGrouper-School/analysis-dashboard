import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import { emptyHostApplication, parseEcosystems } from '#lib/domain/hosting.ts';
import type { HostApplicationInput } from '#lib/domain/types.ts';
import { createMockServices } from './index';

const services = createMockServices({ latency: 0 });
const signInAs = (sub: string) => services.auth.signInWithGoogle(sub);

beforeEach(() => resetDb());

function application(): HostApplicationInput {
	return {
		...emptyHostApplication(),
		institutionName: ' Lalibela Secondary School ',
		contactName: 'Meron D.',
		contactRole: 'Head teacher',
		contactEmail: 'Meron.D@example.org',
		answers: { storage: 'yes', lead: 'yes', connection: 'no', learners: '30', extra: 'dropped' },
		site: {
			placeName: 'Lalibela',
			country: 'Ethiopia',
			ecosystems: ['Highland farmland'],
			learners: 'Grade 9 students.',
			studyGoals: 'When the rains start.'
		}
	};
}

describe('applying to host a Global Classroom (AP-30)', () => {
	it('asks the configured readiness questions, with no sign-in', async () => {
		await services.auth.signOut();
		const questions = await services.hosting.questions();
		expect(questions.map((q) => q.id)).toEqual([
			'storage',
			'lead',
			'connection',
			'learners',
			'notes'
		]);
	});

	it('saves an application and shows its status by its link', async () => {
		await services.auth.signOut();
		const { application: saved, token } = await services.hosting.apply(application());
		expect(saved).toMatchObject({
			institutionName: 'Lalibela Secondary School',
			contactEmail: 'meron.d@example.org',
			status: 'submitted',
			classroomId: null
		});
		// Only answers to the configured questions are kept.
		expect(saved.answers).not.toHaveProperty('extra');
		expect(saved).not.toHaveProperty('token');
		expect(await services.hosting.status(token)).toMatchObject({
			id: saved.id,
			status: 'submitted'
		});
		await expect(services.hosting.status('host_nope')).rejects.toMatchObject({ code: 'not_found' });
	});

	it('refuses an application with missing answers, by field', async () => {
		const input = application();
		input.answers = { storage: 'maybe', learners: 'many' };
		input.site.ecosystems = [];
		input.contactEmail = 'not-an-email';
		await expect(services.hosting.apply(input)).rejects.toMatchObject({
			code: 'invalid_input',
			fields: {
				contactEmail: 'Enter a valid email address.',
				'answers.storage': 'Choose yes or no.',
				'answers.lead': 'Answer this question.',
				'answers.learners': 'Enter a number.',
				'site.ecosystems': 'Name at least one ecosystem.'
			}
		});
	});

	it('lets only platform admins see and decide applications', async () => {
		await signInAs('g-marcus');
		await expect(services.hosting.list()).rejects.toMatchObject({ code: 'forbidden' });
		await expect(services.hosting.decide('happ_andros', true)).rejects.toMatchObject({
			code: 'forbidden'
		});
	});

	it('accepts an application once, then a classroom is created from it once', async () => {
		const { token } = await services.hosting.apply(application());
		await signInAs('g-leila');
		const list = await services.hosting.list();
		expect(list[0].institutionName).toBe('Lalibela Secondary School');
		const andros = list.find((a) => a.id === 'happ_andros')!;
		// Not before it is accepted.
		await expect(
			services.classrooms.create({
				name: 'Andros Classroom',
				institutionName: andros.institutionName,
				headEmail: andros.contactEmail,
				applicationId: andros.id
			})
		).rejects.toMatchObject({ code: 'conflict' });
		expect((await services.hosting.decide(andros.id, true)).status).toBe('accepted');
		await expect(services.hosting.decide(andros.id, false)).rejects.toMatchObject({
			code: 'conflict'
		});
		const classroom = await services.classrooms.create({
			name: 'Andros Classroom',
			institutionName: andros.institutionName,
			headEmail: andros.contactEmail,
			applicationId: andros.id
		});
		const after = (await services.hosting.list()).find((a) => a.id === andros.id)!;
		expect(after.classroomId).toBe(classroom.id);
		await expect(
			services.classrooms.create({
				name: 'Andros Again',
				institutionName: andros.institutionName,
				headEmail: andros.contactEmail,
				applicationId: andros.id
			})
		).rejects.toMatchObject({ code: 'conflict' });
		// The applicant sees the decision on their link.
		const lalibela = list.find((a) => a.institutionName === 'Lalibela Secondary School')!;
		await services.hosting.decide(lalibela.id, false);
		expect((await services.hosting.status(token)).status).toBe('declined');
	});

	it('splits ecosystems typed as a list', () => {
		expect(parseEcosystems('mangrove, coral reef;\nschool garden, ')).toEqual([
			'mangrove',
			'coral reef',
			'school garden'
		]);
	});
});
