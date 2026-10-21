import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ url }) => {
	const [conditions, parameters] = await Promise.all([
		services.labs.currentConditions(),
		services.registry.listParameters()
	]);
	return {
		conditions,
		parameters,
		selected: url.searchParams.get('device'),
		classroom: url.searchParams.get('classroom')
	};
};
