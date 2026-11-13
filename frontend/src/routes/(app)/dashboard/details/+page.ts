import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageLoad } from './$types';

/** The classroom details page is the Classrooms page now; an old link keeps its query. */
export const load: PageLoad = ({ url }) => {
	redirect(307, resolve('/(app)/dashboard/classrooms') + url.search);
};
