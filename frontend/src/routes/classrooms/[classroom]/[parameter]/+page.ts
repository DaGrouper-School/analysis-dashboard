import { error } from '@sveltejs/kit';
import type { DeviceAlerts } from '#lib/domain/types.ts';
import { services, ServiceError } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/**
 * One parameter of a classroom for visitors (AP-21): its meaning and released values. `?device=`
 * opens one device's readings. Members of the classroom also get each device's alert levels.
 */
export const load: PageLoad = async ({ params, parent, url }) => {
	const { user } = await parent();
	try {
		const page = await services.publicSite.parameter(params.classroom, params.parameter);
		const deviceIds = page.series.filter((s) => s.source.kind === 'device').map((s) => s.source.id);
		// Alert levels are members' only: a visitor's request is refused, so it shows none.
		const alerts = user
			? (
					await Promise.all(
						deviceIds.map((id) => services.labs.listDeviceAlerts(id).catch(() => null))
					)
				).filter((a): a is DeviceAlerts => !!a)
			: [];
		return {
			user,
			...page,
			device: url.searchParams.get('device'),
			alerts,
			typeDefinitions: alerts.length ? await services.registry.listDeviceTypes() : []
		};
	} catch (e) {
		if (e instanceof ServiceError && e.code === 'not_found') error(404, e.message);
		throw e;
	}
};
