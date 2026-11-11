import { services } from '#lib/services/index.ts';
import { selectSensor } from '#lib/sensor-selection.ts';
import type { MemberConsent, RawRow } from '#lib/domain/types.ts';
import type { PageLoad } from './$types';

/** How many recent records the Records tab lists. */
const RECORDS = 8;

/**
 * The dashboard (AP-45): one classroom at a time, laid out like its public page, with alerts,
 * domains, devices, labs, records and completeness as tabs. Sync follows the picked device.
 */
export const load: PageLoad = async ({ url, parent }) => {
	const [{ deviceTypes }, selection, parameters, invitations] = await Promise.all([
		parent(),
		selectSensor(url),
		services.registry.listParameters(),
		services.classrooms.listMyInvitations()
	]);
	const { device, classroom, devices } = selection;
	// Hand-entered devices and field observations are what people record; a station sends its own.
	const manual = devices.filter((d) => deviceTypes.find((t) => t.id === d.type)?.manual);
	const [readings, lab, labs, station, consent, records] = await Promise.all([
		device ? services.labs.deviceReadings(device.id, 60) : [],
		device ? services.labs.get(device.labId) : null,
		classroom ? services.labs.listForClassroom(classroom.id) : [],
		classroom ? services.publicSite.station(classroom.id) : null,
		// Only the head and facilitators may see guardian consent; everyone else sees none.
		classroom ? services.classrooms.listConsent(classroom.id).catch((): MemberConsent[] => []) : [],
		classroom
			? Promise.all(
					[...manual.map((d) => d.id), 'field'].map((sourceId) =>
						services.exports
							.rawValues(
								{ classroomId: classroom.id, sourceId, sort: 'time', order: 'desc' },
								RECORDS
							)
							.then((page) => page.rows)
							.catch((): RawRow[] => [])
					)
				).then((pages) =>
					pages
						.flat()
						.sort((a, b) => b.observedAt.localeCompare(a.observedAt))
						.slice(0, RECORDS)
				)
			: []
	]);
	return {
		...selection,
		readings,
		// Heads, facilitators and community scientists can record data; researchers can't.
		canRecord: lab?.canRecord ?? false,
		labLocation: lab?.lab.location ?? null,
		// Every device's active alerts, evaluated by the backend against the classroom's levels (AP-08).
		alerts: selection.classroomConditions.flatMap((c) =>
			c.alerts.map((alert) => ({ ...alert, device: c.device, observedAt: c.reading.observedAt }))
		),
		parameters,
		invitations,
		labs,
		station,
		waitingForConsent: consent.filter((m) => m.status === 'pending'),
		records
	};
};
