import { error } from '@sveltejs/kit';
import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** The printable guardian consent sheet (AP-38), for the classroom's head and facilitators. */
export const load: PageLoad = async ({ params, url }) => {
	try {
		const sheet = await services.classrooms.consentSheet(params.id);
		const memberId = url.searchParams.get('member');
		return { sheet, memberId, classroomId: params.id, contextClassroomId: params.id };
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') error(404, e.message);
		if (e instanceof ServiceError && e.code === 'forbidden') error(403, e.message);
		throw e;
	}
};
