import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** SAM-23: the shared registry. Platform admins only; everyone else gets an explanation. */
export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	if (!user.platformAdmin) return { allowed: false as const };
	const [parameters, changes, alertDefaults] = await Promise.all([
		services.registry.listParameters(),
		services.registry.listChanges(),
		services.registry.listAlertDefaults()
	]);
	return { allowed: true as const, parameters, changes, alertDefaults };
};
