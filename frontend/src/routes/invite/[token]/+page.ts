import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	try {
		return { invitation: await services.classrooms.getInvitation(params.token), error: null };
	} catch (e) {
		if (!(e instanceof ServiceError)) throw e;
		return { invitation: null, error: e.message };
	}
};
