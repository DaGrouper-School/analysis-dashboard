import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** Explore data (AP-21): every classroom on the map with its core readings. No sign-in. */
export const load: PageLoad = async ({ parent }) => {
	const [{ user }, home] = await Promise.all([parent(), services.publicSite.home()]);
	return { user, ...home };
};
