import { error } from '@sveltejs/kit';
import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/**
 * A classroom's public page (AP-21, AP-29, AP-45): its station overview and charts, released
 * data only, with its domains, completeness and About as tabs.
 */
export const load: PageLoad = async ({ params, parent }) => {
	const { user } = await parent();
	try {
		const [station, about] = await Promise.all([
			services.publicSite.station(params.classroom),
			services.publicSite.profile(params.classroom)
		]);
		return { user, ...station, about };
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') error(404, e.message);
		throw e;
	}
};
