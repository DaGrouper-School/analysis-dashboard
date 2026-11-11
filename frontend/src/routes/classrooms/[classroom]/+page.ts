import { error } from '@sveltejs/kit';
import { readFilter } from '#lib/features/classroom/directory.ts';
import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/**
 * The public Classrooms page (AP-21, AP-29, AP-45), open on one classroom: every classroom as a
 * tab, narrowed by the region, country and stage filter, then the classroom's overview. Its
 * domains and other sections are for members only, on the dashboard's classroom details page.
 * Released data only.
 */
export const load: PageLoad = async ({ params, parent, url }) => {
	const { user } = await parent();
	try {
		const station = await services.publicSite.station(params.classroom);
		return { user, ...station, filter: readFilter(url.searchParams) };
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') error(404, e.message);
		throw e;
	}
};
