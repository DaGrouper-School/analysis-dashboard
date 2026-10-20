import { services } from '#lib/services/index.ts';

/**
 * The classroom and sensor picked with `?classroom=` and `?device=`, shared by the dashboard and
 * its raw readings page. Without a pick it opens on the first weather station with a reading,
 * since weather stations carry the headline parameters.
 */
export async function selectSensor(url: URL) {
	const [mine, conditions] = await Promise.all([
		services.classrooms.listMine(),
		services.labs.currentConditions()
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

	return {
		classrooms,
		classroom,
		devices,
		device,
		// Latest reading of each device in the classroom.
		classroomConditions: conditions.filter((c) => devices.some((d) => d.id === c.device.id))
	};
}

/** The query that keeps a classroom and sensor picked across pages. */
export function sensorQuery(classroomId: string, deviceId?: string) {
	let query = `?classroom=${encodeURIComponent(classroomId)}`;
	if (deviceId) query += `&device=${encodeURIComponent(deviceId)}`;
	return query;
}
