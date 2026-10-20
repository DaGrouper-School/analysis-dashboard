import { error } from '@sveltejs/kit';
import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, depends }) => {
	depends('app:classroom');
	try {
		const [detail, labs] = await Promise.all([
			services.classrooms.get(params.id),
			services.labs.listForClassroom(params.id)
		]);
		return { detail, labs };
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') error(404, e.message);
		throw e;
	}
};
