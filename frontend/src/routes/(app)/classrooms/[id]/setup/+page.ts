import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { sensorQuery } from '#lib/utils/sensor-selection.ts';
import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** The institution setup (AP-40), for the classroom's head; anyone else goes to the dashboard. */
export const load: PageLoad = async ({ params }) => {
	const [form, setup, labs] = await Promise.all([
		services.classrooms.institutionSetupForm(),
		services.classrooms.institutionSetup(params.id).catch(() => null),
		services.labs.listForClassroom(params.id).catch(() => [])
	]);
	if (!setup) redirect(307, resolve('/(app)/dashboard') + sensorQuery(params.id));
	return { form, setup, hasLab: labs.length > 0 };
};
