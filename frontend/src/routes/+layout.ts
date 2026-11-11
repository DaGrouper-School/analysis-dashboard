import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { services } from '#lib/services/index.ts';
import { sectionQuery, sensorQuery } from '#lib/features/devices/sensor-selection.ts';
import type { LayoutLoad } from './$types';

// Front-end only for now: render in the browser so the mock layer can use localStorage.
// Revisit when the SAM backend and real auth (HTTP-only session cookie) arrive.
export const ssr = false;
export const prerender = false;

/**
 * The public site's pages that a signed-in member sees in the app instead (AP-45): the public
 * site stays public, and members always work in the app. Links people are sent (invitations,
 * consent), contributor pages and hosting stay where they are. A value page opens in the app.
 */
const IN_THE_APP = new Set(['/', '/classrooms']);
const CLASSROOM_PAGES = new Set(['/classrooms/[classroom]']);
/**
 * A classroom page's `?tab=` as a dashboard section. The members' classroom page, folded into
 * the dashboard, had labs, members, invitations and consent tabs at the same address.
 */
const SECTION: Record<string, string> = {
	about: 'about',
	domains: 'domains',
	complete: 'complete',
	labs: 'labs',
	members: 'people',
	invitations: 'people',
	consent: 'people'
};

export const load: LayoutLoad = async ({ depends, route, params, url }) => {
	depends('app:session');
	const user = await services.auth.getCurrentUser();
	if (user && route.id && IN_THE_APP.has(route.id)) redirect(307, resolve('/(app)/dashboard'));
	if (user && route.id && CLASSROOM_PAGES.has(route.id) && params.classroom) {
		const section = SECTION[url.searchParams.get('tab') ?? ''];
		redirect(
			307,
			resolve('/(app)/dashboard') +
				(section ? sectionQuery(params.classroom, section) : sensorQuery(params.classroom))
		);
	}
	if (user && route.id === '/classrooms/[classroom]/[parameter]' && params.parameter) {
		redirect(
			307,
			resolve('/(app)/dashboard/[classroom]/[parameter]', {
				classroom: params.classroom!,
				parameter: params.parameter
			}) + url.search
		);
	}
	return { user };
};
