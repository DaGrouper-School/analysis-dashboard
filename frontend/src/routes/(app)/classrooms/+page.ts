import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	return {
		classrooms: await services.classrooms.listMine(),
		canCreate: user.platformAdmin || user.role === 'institution_head'
	};
};
