import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import { services } from '#lib/services/index.ts';
import { nextRoute } from '#lib/routing.ts';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ parent }) => {
	const { user } = await parent();
	if (!user?.onboardingCompleted) redirect(307, resolve(nextRoute(user)));
	// The shared registry: every page can name device types and domains.
	const [deviceTypes, domains] = await Promise.all([
		services.registry.listDeviceTypes(),
		services.registry.listDomains()
	]);
	return { user, deviceTypes, domains };
};
