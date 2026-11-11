import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { sectionQuery } from '#lib/utils/sensor-selection.ts';
import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, url }) => {
	const detail = await services.classrooms.get(params.id);
	if (!detail.permissions.invite.length) {
		redirect(307, resolve('/(app)/dashboard') + sectionQuery(params.id, 'labs'));
	}
	return { classroom: detail.classroom, onboarding: url.searchParams.has('onboarding') };
};
