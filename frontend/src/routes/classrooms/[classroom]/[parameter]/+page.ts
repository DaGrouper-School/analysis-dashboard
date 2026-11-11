import { error } from '@sveltejs/kit';
import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** One parameter of a classroom for visitors (AP-21): its meaning and released values. */
export const load: PageLoad = async ({ params, parent }) => {
	const { user } = await parent();
	try {
		return {
			user,
			...(await services.publicSite.parameter(params.classroom, params.parameter))
		};
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') error(404, e.message);
		throw e;
	}
};
