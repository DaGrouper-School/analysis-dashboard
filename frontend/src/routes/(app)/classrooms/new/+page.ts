import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** A new classroom, set up from an accepted host application (AP-30) when one is named. */
export const load: PageLoad = async ({ parent, url }) => {
	const { user } = await parent();
	const id = url.searchParams.get('application');
	if (!id || !user.platformAdmin) return { application: null };
	const application = (await services.hosting.list()).find((a) => a.id === id) ?? null;
	return { application };
};
