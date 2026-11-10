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
		// Minors' guardian consent (AP-38): only for the head and facilitators.
		const consent = detail.permissions.manageConsent
			? await services.classrooms.listConsent(params.id)
			: [];
		return { detail, labs, consent };
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') error(404, e.message);
		throw e;
	}
};
