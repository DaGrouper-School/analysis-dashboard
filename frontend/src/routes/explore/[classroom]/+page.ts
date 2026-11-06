import { error } from '@sveltejs/kit';
import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** A classroom's station overview for visitors (AP-21): released data only. */
export const load: PageLoad = async ({ params, parent }) => {
	const { user } = await parent();
	try {
		return { user, ...(await services.publicSite.station(params.classroom)) };
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') error(404, e.message);
		throw e;
	}
};
