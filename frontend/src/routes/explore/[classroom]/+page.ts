import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageLoad } from './$types';

/** A classroom's station, now its public classroom page (AP-45). */
export const load: PageLoad = ({ params, url }) => {
	redirect(308, resolve('/classrooms/[classroom]', { classroom: params.classroom }) + url.search);
};
