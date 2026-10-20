import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => {
	const [conditions, recent, parameters, invitations] = await Promise.all([
		services.labs.currentConditions(),
		services.labs.recentReadings(12),
		services.registry.listParameters(),
		services.classrooms.listMyInvitations()
	]);
	// Conditions only come from devices in the user's own classrooms, newest reading first.
	// Weather stations lead, since they carry the headline parameters.
	const featured = conditions.find((c) => c.device.type === 'weather_station') ?? null;
	return {
		featured,
		others: conditions.filter((c) => c !== featured),
		recent,
		parameters,
		invitations
	};
};
