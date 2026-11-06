import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** Host applications (AP-30): platform admins review them; everyone else gets an explanation. */
export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	if (!user.platformAdmin) return { allowed: false as const };
	const [applications, questions] = await Promise.all([
		services.hosting.list(),
		services.hosting.questions()
	]);
	return { allowed: true as const, applications, questions };
};
