import { error } from '@sveltejs/kit';
import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** A public section or policy page that is not built yet (AP-20). */
export const load: PageLoad = async ({ params, parent }) => {
	const [{ user }, content] = await Promise.all([parent(), services.publicSite.content()]);
	const section = [...content.sections, ...content.policies].find((s) => s.id === params.id);
	if (!section) error(404, 'Page not found');
	return { user, content, section };
};
