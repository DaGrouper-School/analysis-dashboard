import { error } from '@sveltejs/kit';
import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/** A contributor's public profile, by participant ID (AP-39). */
export const load: PageLoad = async ({ params, parent }) => {
	const { user } = await parent();
	const participantId = Number(params.participantId);
	if (!Number.isInteger(participantId)) error(404, 'Contributor not found.');
	try {
		return { user, ...(await services.publicSite.contributor(participantId)) };
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') error(404, e.message);
		throw e;
	}
};
