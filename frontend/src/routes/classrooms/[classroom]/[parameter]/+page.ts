import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageLoad } from './$types';

/**
 * A value's page is for the classroom's members, in the app (the root layout sends anyone signed
 * in there). A visitor's old link opens the classroom's public page.
 */
export const load: PageLoad = ({ params }) => {
	redirect(307, resolve('/classrooms/[classroom]', { classroom: params.classroom }));
};
