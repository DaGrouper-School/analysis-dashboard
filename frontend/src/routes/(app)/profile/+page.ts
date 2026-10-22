import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

/** The profile is part of Settings now. */
export const load: PageLoad = () => {
	redirect(307, resolve('/(app)/settings'));
};
