import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	if (!user.platformAdmin && user.role !== 'institution_head') {
		redirect(307, resolve('/(app)/classrooms'));
	}
};
