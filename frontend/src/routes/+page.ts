import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** The public home page (AP-20): open to everyone, signed in or not. */
export const load: PageLoad = async ({ parent }) => {
	const [{ user }, home] = await Promise.all([parent(), services.publicSite.home()]);
	return { user, ...home };
};
