import { alertsFor } from '#lib/domain/alerts.ts';
import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/**
 * The dashboard shows one classroom and one of its sensors at a time, picked with
 * `?classroom=` and `?device=`. Without a pick it opens on the first weather station with a
 * reading, since weather stations carry the headline parameters.
 */
export const load: PageLoad = async ({ url }) => {
	const [mine, conditions, parameters, invitations] = await Promise.all([
		services.classrooms.listMine(),
		services.labs.currentConditions(),
		services.registry.listParameters(),
		services.classrooms.listMyInvitations()
	]);
	// Only classrooms the user belongs to: a platform admin lists every classroom but sees no data.
	const classrooms = mine.filter((c) => c.myRole !== null);
	const fallback =
		conditions.find((c) => c.device.type === 'weather_station') ?? conditions[0] ?? null;
	const classroom =
		classrooms.find((c) => c.id === url.searchParams.get('classroom')) ??
		classrooms.find((c) => c.id === fallback?.reading.classroomId) ??
		classrooms[0] ??
		null;

	const devices = classroom ? await services.labs.listClassroomDevices(classroom.id) : [];
	const withReadings = new Set(conditions.map((c) => c.device.id));
	const device =
		devices.find((d) => d.id === url.searchParams.get('device')) ??
		devices.find((d) => d.type === 'weather_station' && withReadings.has(d.id)) ??
		devices.find((d) => withReadings.has(d.id)) ??
		devices[0] ??
		null;
	const readings = device ? await services.labs.deviceReadings(device.id, 60) : [];

	return {
		classrooms,
		classroom,
		devices,
		device,
		readings,
		alerts: readings[0] ? alertsFor(readings[0]) : [],
		// Latest reading of each device in the classroom, for the globe and the device cards.
		classroomConditions: conditions.filter((c) => devices.some((d) => d.id === c.device.id)),
		parameters,
		invitations
	};
};
