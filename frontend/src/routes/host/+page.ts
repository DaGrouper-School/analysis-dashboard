import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** Applying to host a Global Classroom (AP-30): open to anyone, no sign-in. */
export const load: PageLoad = async ({ parent }) => {
	const [{ user }, content, questions] = await Promise.all([
		parent(),
		services.publicSite.content(),
		services.hosting.questions()
	]);
	return { user, content, questions };
};
