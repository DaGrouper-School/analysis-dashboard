import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import { nextRoute } from '#lib/routing.ts';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ parent }) => {
	const { user } = await parent();
	if (!user?.onboardingCompleted) redirect(307, resolve(nextRoute(user)));
	return { user };
};
