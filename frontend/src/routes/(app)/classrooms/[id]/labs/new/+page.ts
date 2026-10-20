import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, url }) => {
	const detail = await services.classrooms.get(params.id);
	if (!detail.permissions.invite.length) {
		redirect(307, resolve('/(app)/classrooms/[id]', { id: params.id }));
	}
	return { classroom: detail.classroom, onboarding: url.searchParams.has('onboarding') };
};
