import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import { nextRoute } from '#lib/routing.ts';
import type { LayoutLoad } from './$types';

// Signed-in users skip sign-up (AP-01: existing users go straight to /dashboard).
export const load: LayoutLoad = async ({ parent }) => {
	const { user } = await parent();
	if (user) redirect(307, resolve(nextRoute(user)));
};
