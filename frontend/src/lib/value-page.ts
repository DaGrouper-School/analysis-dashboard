import { error } from '@sveltejs/kit';
import type { DeviceAlerts, User } from '#lib/domain/types.ts';
import { services, ServiceError } from '#lib/services/index.ts';

/**
 * What a value's page shows (AP-23), in the public site for visitors and in the app for anyone
 * signed in: its meaning and values, `?device=` to open one device's readings, and, for members
 * of the classroom, each device's alert levels.
 */
export async function loadValuePage(
	classroomId: string,
	parameterId: string,
	url: URL,
	user: User | null
) {
	try {
		const page = await services.publicSite.parameter(classroomId, parameterId);
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
}

export type ValuePageData = Awaited<ReturnType<typeof loadValuePage>>;
