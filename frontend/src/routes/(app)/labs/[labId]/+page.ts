import { error } from '@sveltejs/kit';
import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, url, depends }) => {
	depends('app:lab');
	try {
		return {
			detail: await services.labs.get(params.labId),
			onboarding: url.searchParams.has('onboarding')
		};
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') error(404, e.message);
		throw e;
	}
};
