import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageLoad } from './$types';

/** A value's page lives under Domains now; an old link opens it there, with its query. */
export const load: PageLoad = ({ params, url }) => {
	redirect(
		307,
		resolve('/(app)/dashboard/domains/[classroom]/[parameter]', {
			classroom: params.classroom,
			parameter: params.parameter
		}) + url.search
	);
};
