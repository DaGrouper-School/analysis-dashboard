import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import { contextClassroom } from '#lib/classroom-context.ts';
import { services } from '#lib/services/index.ts';
import { nextRoute } from '#lib/routing.ts';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ parent, url, depends }) => {
	const { user } = await parent();
	if (!user?.onboardingCompleted) redirect(307, resolve(nextRoute(user)));
	depends('app:classroom-context');
	// The shared registry: every page can name device types and domains.
	const [deviceTypes, domains, mine] = await Promise.all([
		services.registry.listDeviceTypes(),
		services.registry.listDomains(),
		services.classrooms.listMine()
	]);
	// The classrooms the top bar picks from: ones the user belongs to, as those have data.
	const memberClassrooms = mine.filter((c) => c.myRole !== null);
	return {
		user,
		deviceTypes,
		domains,
		memberClassrooms,
		contextClassroom: contextClassroom(memberClassrooms, url)
	};
};
