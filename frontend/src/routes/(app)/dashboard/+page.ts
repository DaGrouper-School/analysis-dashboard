import { services } from '#lib/services/index.ts';
import { selectSensor } from '#lib/sensor-selection.ts';
import type { PageLoad } from './$types';

/** The dashboard shows one classroom and one of its sensors at a time. */
export const load: PageLoad = async ({ url }) => {
	const [selection, parameters, invitations] = await Promise.all([
		selectSensor(url),
		services.registry.listParameters(),
		services.classrooms.listMyInvitations()
	]);
	const { device } = selection;
	const [readings, lab, deviceAlerts] = await Promise.all([
		device ? services.labs.deviceReadings(device.id, 60) : [],
		device ? services.labs.get(device.labId) : null,
		device ? services.labs.listDeviceAlerts(device.id) : null
	]);
	return {
		...selection,
		readings,
		// Heads, facilitators and community scientists can record data; researchers can't.
		canRecord: lab?.canRecord ?? false,
		labLocation: lab?.lab.location ?? null,
		// Evaluated by the backend against the classroom's levels (AP-08).
		alerts: selection.classroomConditions.find((c) => c.device.id === device?.id)?.alerts ?? [],
		alertRules: deviceAlerts?.rules ?? [],
		parameters,
		invitations
	};
};
