import { beforeEach, describe, expect, it, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { resetDb } from '#lib/data/mock-db.ts';
import { services } from '#lib/services/index.ts';
import { pngFile } from '../../../test/files.ts';
import InstitutionSetupWizard from './InstitutionSetupWizard.svelte';

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

async function renderWizard(classroomId: string) {
	const [form, setup] = await Promise.all([
		services.classrooms.institutionSetupForm(),
		services.classrooms.institutionSetup(classroomId)
	]);
	const ondone = vi.fn();
	const onlater = vi.fn();
	render(InstitutionSetupWizard, { form, setup, ondone, onlater });
	return { ondone, onlater };
}

describe('institution setup wizard (AP-40)', () => {
	it('lets the head of a new classroom add a photo', async () => {
		const id = await newClassroomWithHead();
		await renderWizard(id);

		await expect.element(page.getByText('No photo yet')).toBeVisible();
		await userEvent.upload(page.getByLabelText('Photo of your institution'), await pngFile());
		await expect.element(page.getByRole('img', { name: 'The new photo' })).toBeVisible();
	});

	it('asks for the required answers before moving on', async () => {
		const id = await newClassroomWithHead();
		await renderWizard(id);

		await page.getByRole('button', { name: /^Next/ }).click();
		await expect.element(page.getByText('Choose a photo.')).toBeVisible();
		await expect.element(page.getByText('Answer this question.')).toBeVisible();
	});

	it('saves each step and resumes there after "Finish later"', async () => {
		const id = await newClassroomWithHead();
		const { onlater } = await renderWizard(id);

		await userEvent.upload(page.getByLabelText('Photo of your institution'), await pngFile());
		await page.getByLabelText('Describe the photo').fill('The school on the harbour');
		await page.getByRole('button', { name: /^Next/ }).click();
		await expect.element(page.getByLabelText('Tell us your institution’s story')).toBeVisible();
		await page.getByRole('button', { name: 'Finish later' }).click();

		await vi.waitFor(() => expect(onlater).toHaveBeenCalled());
		const setup = await services.classrooms.institutionSetup(id);
		expect(setup.step).toBe(1);
		expect(setup.answers.photo).toMatch(/^data:image\/jpeg;base64,/);
		expect(setup.answers.photoAlt).toBe('The school on the harbour');
		expect(setup.completedAt).toBeNull();
	});

	it('finishes, and fills in the institution overview', async () => {
		const id = await newClassroomWithHead();
		const { ondone } = await renderWizard(id);
		const next = () => page.getByRole('button', { name: /^Next/ }).click();

		await userEvent.upload(page.getByLabelText('Photo of your institution'), await pngFile());
		await page.getByLabelText('Describe the photo').fill('The school on the harbour');
		await next();
		await page.getByLabelText('When was your institution founded?').fill('1962');
		await page
			.getByLabelText('Tell us your institution’s story')
			.fill('A harbour school whose students track storms on the shoreline.');
		await page.getByLabelText('Shoreline').click();
		await page.getByLabelText(/^Other/).fill('Seagrass');
		await next();
		await page.getByLabelText('Partner or hub organisation').fill('Earth Care');
		await next();
		await page.getByLabelText('Yes').click();
		await next();
		await expect.element(page.getByText('Shoreline, Seagrass')).toBeVisible();
		await page.getByRole('button', { name: 'Finish setup' }).click();

		await vi.waitFor(() => expect(ondone).toHaveBeenCalled());
		const setup = await services.classrooms.institutionSetup(id);
		expect(setup.completedAt).not.toBeNull();
		expect(setup.answers).toMatchObject({
			photoAlt: 'The school on the harbour',
			foundedYear: '1962',
			ecosystems: ['Shoreline', 'Seagrass'],
			hub: 'Earth Care',
			showPublicly: 'yes'
		});
		expect((await services.classrooms.get(id)).setup?.institutionDone).toBe(true);
	});
});
