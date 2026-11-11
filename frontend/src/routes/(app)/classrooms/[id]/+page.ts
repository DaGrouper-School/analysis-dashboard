import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { sectionQuery } from '#lib/sensor-selection.ts';
import type { PageLoad } from './$types';

/** The members' classroom page folded into the dashboard (AP-45): its tabs are dashboard tabs. */
const SECTION: Record<string, string> = {
	labs: 'labs',
	members: 'people',
	invitations: 'people',
	consent: 'people'
};

export const load: PageLoad = ({ params, url }) => {
	const section = SECTION[url.searchParams.get('tab') ?? ''] ?? 'people';
	redirect(307, resolve('/(app)/dashboard') + sectionQuery(params.id, section));
};
