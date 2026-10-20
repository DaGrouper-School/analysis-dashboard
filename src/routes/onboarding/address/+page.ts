import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import { isProfileStepDone } from '#lib/routing.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	if (!isProfileStepDone(user)) redirect(307, resolve('/onboarding/profile'));
};
