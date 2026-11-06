import { readFilter } from '#lib/directory.ts';
import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** The Global Classrooms directory (AP-28): every classroom by region. No sign-in. */
export const load: PageLoad = async ({ parent, url }) => {
	const [{ user }, home] = await Promise.all([parent(), services.publicSite.home()]);
	return { user, ...home, filter: readFilter(url.searchParams) };
};
