import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ parent }) => {
	const { user } = await parent();
	if (!user) redirect(307, resolve('/(auth)/signup'));
	if (user.onboardingCompleted) redirect(307, resolve('/(app)/dashboard'));
	return { user };
};
