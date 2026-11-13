import { beforeEach, describe, expect, it } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { resetDb } from '#lib/data/mock-db.ts';
import { services } from '#lib/services/index.ts';
import ClassroomSettings from './ClassroomSettings.svelte';
import SetupReminder from './SetupReminder.svelte';

beforeEach(() => resetDb());

/** A platform admin sets up a classroom; Sarah joins it as its institution head. */
async function newClassroomWithHead() {
	await services.auth.signInWithGoogle('g-leila');
	const created = await services.classrooms.create({
		name: 'Harbour Classroom',
		institutionName: 'Bay Street School',
		headEmail: 'sarah.k@example.org'
	});
	const { invitations } = await services.classrooms.get(created.id);
	await services.auth.signInWithGoogle('g-sarah');
	await services.classrooms.acceptInvitation(invitations[0].token);
	return created.id;
}

describe('classroom settings (AP-40)', () => {
	it('sends the head of a new classroom to finish the institution setup', async () => {
		const id = await newClassroomWithHead();
		render(ClassroomSettings, { detail: await services.classrooms.get(id) });

		const link = page.getByRole('link', { name: 'Continue setup' });
		await expect.element(link).toHaveAttribute('href', `/classrooms/${id}/setup`);
		await expect.element(page.getByText('Not signed yet.', { exact: false })).toBeVisible();
		await expect
			.element(page.getByRole('button', { name: 'Mark as signed' }))
			.not.toBeInTheDocument();
	});

	it('lets a platform admin mark the host agreement signed', async () => {
		const id = await newClassroomWithHead();
		await services.auth.signInWithGoogle('g-leila');
		render(ClassroomSettings, { detail: await services.classrooms.get(id) });

		await page.getByRole('button', { name: 'Mark as signed' }).click();
		await expect
			.poll(async () => (await services.classrooms.get(id)).setup?.hostAgreementSigned)
			.toBe(true);
	});

	it('reminds the head who chose "Finish later"', async () => {
		render(SetupReminder, { classroomId: 'cls_x', institutionName: 'Bay Street School' });
		await expect.element(page.getByText('Finish setting up Bay Street School')).toBeVisible();
		await expect
			.element(page.getByRole('link', { name: /Continue setup/ }))
			.toHaveAttribute('href', '/classrooms/cls_x/setup');
	});
});
