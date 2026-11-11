import { error, redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { sensorQuery } from '#lib/sensor-selection.ts';
import type { DeviceAlerts } from '#lib/domain/types.ts';
import type { PageLoad } from './$types';
import { services, ServiceError } from '#lib/services/index.ts';

/**
 * What a value's page shows (AP-23): its meaning and values, `?device=` to open one device's
 * readings, and each device's alert levels. For the classroom's members only: anyone else goes
 * back to the classroom's overview.
 */
export const load: PageLoad = async ({ params, parent, url }) => {
	const { user } = await parent();
	const { classroom: classroomId, parameter: parameterId } = params;
	try {
		const page = await services.publicSite.parameter(classroomId, parameterId);
		if (!page.member) redirect(307, resolve('/(app)/dashboard') + sensorQuery(classroomId));
		const deviceIds = page.series.filter((s) => s.source.kind === 'device').map((s) => s.source.id);
		const alerts = (
			await Promise.all(deviceIds.map((id) => services.labs.listDeviceAlerts(id).catch(() => null)))
		).filter((a): a is DeviceAlerts => !!a);
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
