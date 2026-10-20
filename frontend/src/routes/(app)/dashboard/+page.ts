import { alertsFor } from '#lib/domain/alerts.ts';
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
	const [readings, lab] = device
		? await Promise.all([
				services.labs.deviceReadings(device.id, 60),
				services.labs.get(device.labId)
			])
		: [[], null];
	return {
		...selection,
		readings,
		// Heads, facilitators and community scientists can record data; researchers can't.
		canRecord: lab?.canRecord ?? false,
		labLocation: lab?.lab.location ?? null,
		alerts: readings[0] ? alertsFor(readings[0]) : [],
		parameters,
		invitations
	};
};
