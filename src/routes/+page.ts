import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import { nextRoute } from '#lib/routing.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	redirect(307, resolve(nextRoute(user)));
};
