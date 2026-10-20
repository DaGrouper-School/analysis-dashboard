import { error } from '@sveltejs/kit';
import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	try {
		const detail = await services.labs.get(params.labId);
		if (!detail.canManage)
			error(403, 'Only facilitators and the institution head can edit the lab.');
		return { lab: detail.lab };
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') error(404, e.message);
		throw e;
	}
};
