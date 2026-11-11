import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import type { PageLoad } from './$types';

/** The old domain view: each value now has its own page, so old links open the dashboard's Domains tab. */
export const load: PageLoad = ({ url }) => {
	const query = new URLSearchParams();
	for (const key of ['classroom', 'device'])
		if (url.searchParams.has(key)) query.set(key, url.searchParams.get(key)!);
	query.set('tab', 'domains');
	redirect(308, `${resolve('/(app)/dashboard')}?${query}`);
};
