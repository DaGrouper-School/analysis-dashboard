import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

/**
 * Classrooms in the nav: the page of the classroom picked on the dashboard. With none yet, the
 * classroom list in Settings, where one is created or joined.
 */
export const load: PageLoad = async ({ parent }) => {
	const { contextClassroom } = await parent();
	if (contextClassroom) {
		redirect(307, resolve('/(app)/classrooms/[id]', { id: contextClassroom.id }));
	}
	redirect(307, resolve('/(app)/settings') + '?tab=classrooms');
};
