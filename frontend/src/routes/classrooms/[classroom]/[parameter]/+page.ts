import { loadValuePage } from '#lib/value-page.ts';
import type { PageLoad } from './$types';

/** One value of a classroom for visitors (AP-21). Anyone signed in sees it in the app. */
export const load: PageLoad = async ({ params, parent, url }) => {
	const { user } = await parent();
	return loadValuePage(params.classroom, params.parameter, url, user);
};
