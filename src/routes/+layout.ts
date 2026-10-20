import { services } from '#lib/services/index.ts';
import type { LayoutLoad } from './$types';

// Front-end only for now: render in the browser so the mock layer can use localStorage.
// Revisit when the SAM backend and real auth (HTTP-only session cookie) arrive.
export const ssr = false;
export const prerender = false;

export const load: LayoutLoad = async ({ depends }) => {
	depends('app:session');
	const user = await services.auth.getCurrentUser();
	return { user };
};
