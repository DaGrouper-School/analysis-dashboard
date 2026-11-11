import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { services } from '#lib/services/index.ts';
import { sensorQuery } from '#lib/sensor-selection.ts';
import type { LayoutLoad } from './$types';

// Front-end only for now: render in the browser so the mock layer can use localStorage.
// Revisit when the SAM backend and real auth (HTTP-only session cookie) arrive.
export const ssr = false;
export const prerender = false;

/**
 * The public site's pages that a signed-in member sees in the app instead (AP-45): the public
 * site stays public, and members always work in the app. Links people are sent (invitations,
 * consent), contributor pages, hosting and value pages stay where they are.
 */
const IN_THE_APP = new Set(['/', '/explore', '/global-classrooms']);
const CLASSROOM_PAGES = new Set(['/explore/[classroom]', '/global-classrooms/[classroom]']);

export const load: LayoutLoad = async ({ depends, route, params }) => {
	depends('app:session');
	const user = await services.auth.getCurrentUser();
	if (user && route.id && IN_THE_APP.has(route.id)) redirect(307, resolve('/(app)/dashboard'));
	if (user && route.id && CLASSROOM_PAGES.has(route.id) && params.classroom) {
		redirect(307, resolve('/(app)/dashboard') + sensorQuery(params.classroom));
	}
	return { user };
};
