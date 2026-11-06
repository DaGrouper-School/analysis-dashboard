import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** A guardian's consent link (AP-04): open to anyone who has it, no sign-in. */
export const load: PageLoad = async ({ params, parent }) => {
	const [{ user }, content] = await Promise.all([parent(), services.publicSite.content()]);
	try {
		return {
			user,
			content,
			token: params.token,
			request: await services.consent.get(params.token)
		};
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') {
			return { user, content, token: params.token, request: null };
		}
		throw e;
	}
};
