import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/**
 * AP-16: a guided form for a manual observation. `?lab=`, `?device=` and `?template=` preselect
 * what the dashboard or lab page was showing.
 */
export const load: PageLoad = async ({ url }) => {
	const [targets, templates, parameters] = await Promise.all([
		services.labs.observationTargets(),
		services.registry.listObservationTemplates(),
		services.registry.listParameters()
	]);
	return {
		targets,
		templates,
		parameters,
		asked: {
			lab: url.searchParams.get('lab'),
			device: url.searchParams.get('device'),
			template: url.searchParams.get('template')
		}
	};
};
