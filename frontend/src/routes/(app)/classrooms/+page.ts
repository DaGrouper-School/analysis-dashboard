import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

/** Classrooms live under Settings now. */
export const load: PageLoad = () => {
	redirect(307, resolve('/(app)/settings') + '?tab=classrooms');
};
