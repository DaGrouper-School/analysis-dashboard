import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** The applicant's status link (AP-30): open to anyone who has it, no sign-in. */
export const load: PageLoad = async ({ params, parent }) => {
	const [{ user }, content] = await Promise.all([parent(), services.publicSite.content()]);
	try {
		return { user, content, application: await services.hosting.status(params.token) };
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') {
			return { user, content, application: null };
		}
		throw e;
	}
};
