import { services, ServiceError } from '#lib/services/index.ts';
import { loadClassroomMarkers } from '#lib/globe-markers.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const markers = loadClassroomMarkers();
	try {
		return {
			invitation: await services.classrooms.getInvitation(params.token),
			error: null,
			markers
		};
	} catch (e) {
		if (!(e instanceof ServiceError)) throw e;
		return { invitation: null, error: e.message, markers };
	}
};
