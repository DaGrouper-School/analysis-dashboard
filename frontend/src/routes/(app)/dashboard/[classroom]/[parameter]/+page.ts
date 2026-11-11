import { loadValuePage } from '#lib/value-page.ts';
import type { PageLoad } from './$types';

/** One value of a classroom in the app: the same page as the public one, in the app's frame. */
export const load: PageLoad = async ({ params, parent, url }) => {
	const { user } = await parent();
	return loadValuePage(params.classroom, params.parameter, url, user ?? null);
};
