import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageLoad } from './$types';

/** Explore data merged into the public Classrooms page (AP-45); old links keep working. */
export const load: PageLoad = ({ url }) => {
	redirect(308, resolve('/classrooms') + url.search);
};
