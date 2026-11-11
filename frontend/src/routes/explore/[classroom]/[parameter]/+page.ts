import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageLoad } from './$types';

/** A value page, at its new address under the public classroom page (AP-45). */
export const load: PageLoad = ({ params, url }) => {
	redirect(
		308,
		resolve('/classrooms/[classroom]/[parameter]', {
			classroom: params.classroom,
			parameter: params.parameter
		}) + url.search
	);
};
