import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageLoad } from './$types';

/** A classroom's profile, now the About tab of its public classroom page (AP-45). */
export const load: PageLoad = ({ params }) => {
	redirect(308, resolve('/classrooms/[classroom]', { classroom: params.classroom }) + '?tab=about');
};
