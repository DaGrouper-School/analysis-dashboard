import { error } from '@sveltejs/kit';
import { rememberClassroom } from '#lib/classroom-context.ts';
import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, url, depends }) => {
	depends('app:lab');
	try {
		const [detail, parameters, templates] = await Promise.all([
			services.labs.get(params.labId),
			services.registry.listParameters(),
			services.registry.listObservationTemplates()
		]);
		// Opening a lab puts its classroom in context, so the top bar and Labs follow it.
		rememberClassroom(detail.lab.classroomId);
		return {
			contextClassroomId: detail.lab.classroomId,
			detail,
			parameters,
			templates,
			onboarding: url.searchParams.has('onboarding')
		};
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') error(404, e.message);
		throw e;
	}
};
