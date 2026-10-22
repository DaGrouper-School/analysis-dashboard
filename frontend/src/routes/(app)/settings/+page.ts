import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	const [classrooms, invitations] = await Promise.all([
		services.classrooms.listMine(),
		services.classrooms.listMyInvitations()
	]);
	return {
		classrooms,
		invitations,
		canCreate: user.platformAdmin || user.role === 'institution_head'
	};
};
