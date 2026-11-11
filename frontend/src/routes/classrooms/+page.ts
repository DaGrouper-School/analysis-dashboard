import { readFilter } from '#lib/directory.ts';
import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/**
 * The public Classrooms page (AP-21, AP-28, AP-45): every classroom on the map, their core
 * readings and the directory by region. Replaces Explore data and Global Classrooms. No sign-in.
 */
export const load: PageLoad = async ({ parent, url }) => {
	const [{ user }, home] = await Promise.all([parent(), services.publicSite.home()]);
	return { user, ...home, filter: readFilter(url.searchParams) };
};
