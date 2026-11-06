import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	const [classrooms, invitations, consent] = await Promise.all([
		services.classrooms.listMine(),
		services.classrooms.listMyInvitations(),
		user.minor ? services.profile.getGuardianConsent() : null
	]);
	return {
		classrooms,
		invitations,
		consent,
		// Platform admins create classrooms and invite each one's institution head.
		canCreate: user.platformAdmin
	};
};
