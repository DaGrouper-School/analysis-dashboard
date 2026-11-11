import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageLoad } from './$types';

/** Your classrooms are the dashboard's classroom tabs (AP-45). */
export const load: PageLoad = () => {
	redirect(307, resolve('/(app)/dashboard'));
};
