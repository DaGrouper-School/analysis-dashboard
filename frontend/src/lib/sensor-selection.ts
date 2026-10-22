import { contextClassroom, rememberClassroom } from '#lib/classroom-context.ts';
import { services } from '#lib/services/index.ts';

/**
 * The classroom and sensor picked with `?classroom=` and `?device=`, shared by the dashboard and
 * its domain and raw readings pages. Without `?classroom=` it is the classroom in context, picked
 * in the top bar. Without a pick it opens on the first weather station with a reading,
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
	const classroom = contextClassroom(classrooms, url, fallback?.reading.classroomId);
	if (classroom) rememberClassroom(classroom.id);

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

/**
 * A sensor's domain view, opened on `domain` and filtered to values matching `search`, e.g. from
 * an active alert. The path is the caller's to resolve.
 */
export function domainQuery(
	classroomId: string,
	deviceId: string,
	{ domain, search }: { domain?: string | null; search?: string } = {}
) {
	let query = sensorQuery(classroomId, deviceId);
	if (domain) query += `&domain=${encodeURIComponent(domain)}`;
	if (search) query += `&q=${encodeURIComponent(search)}`;
	return query;
}
