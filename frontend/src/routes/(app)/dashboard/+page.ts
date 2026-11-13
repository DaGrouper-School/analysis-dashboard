import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { services } from '#lib/services/index.ts';
import { selectSensor } from '#lib/utils/sensor-selection.ts';
import { domainsHref } from '#lib/utils/site.ts';
import type { MemberConsent, RawRow } from '#lib/domain/types.ts';
import type { PageLoad } from './$types';
import type { PageLoad as DetailsLoad } from './details/$types';

/** How many recent records the Manual records tab loads, then pages ten at a time. */
const RECORDS = 200;

/**
 * The dashboard (AP-45): one classroom at a time, with the same overview as its public page. Its
 * sections (measurements, labs, records, completeness and the rest) are tabs on the Classrooms
 * page, which loads the same data; its domains have their own page. Sync follows the picked
 * device. Another classroom (`?classroom=` one the user is not in) shows its public data only.
 */
export const load: PageLoad = (event) => {
	// A link to a section (`?tab=`) opens it on the Classrooms page, or the Domains page.
	const tab = event.url.searchParams.get('tab');
	const classroom = event.url.searchParams.get('classroom');
	if (tab === 'domains' && classroom) {
		redirect(307, domainsHref(classroom, event.url.searchParams.get('device')));
	}
	if (tab) redirect(307, resolve('/(app)/dashboard/classrooms') + event.url.search);
	return loadDashboard(event);
};

export async function loadDashboard({
	url,
	parent,
	depends
}: Pick<Parameters<PageLoad>[0] | Parameters<DetailsLoad>[0], 'url' | 'parent' | 'depends'>) {
	depends('app:classroom');
	const [{ deviceTypes }, selection] = await Promise.all([parent(), selectSensor(url)]);
	const { device, classroom, devices } = selection;
	// Another classroom, opened from the globe or a public link: its public data, in the app.
	const asked = url.searchParams.get('classroom');
	const visiting =
		asked && !selection.classrooms.some((c) => c.id === asked)
			? await services.publicSite.station(asked).catch(() => null)
			: null;
	// The About tab: the classroom shown, whether it is the user's or one they are visiting.
	const shownId = visiting?.classroom.id ?? classroom?.id;
	const aboutLoading = shownId ? services.publicSite.profile(shownId).catch(() => null) : null;
	// Hand-entered devices and field observations are what people record; a station sends its own.
	const manual = devices.filter((d) => deviceTypes.find((t) => t.id === d.type)?.manual);
	const [readings, lab, labs, station, consent, records, detail] = await Promise.all([
		// The device's recent readings, for the last GPS position a sync starts from. Its tiles
		// and cards come with the station, the same as on the public page.
		device ? services.labs.deviceReadings(device.id, 50) : [],
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
								{ page: 1, pageSize: RECORDS }
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
			: [],
		// Members, invitations and settings for the People and Settings tabs.
		classroom ? services.classrooms.get(classroom.id) : null
	]);
	const about = await aboutLoading;
	return {
		...selection,
		about,
		visiting,
		readings,
		// Heads, facilitators and community scientists can record data; researchers can't.
		canRecord: lab?.canRecord ?? false,
		labLocation: lab?.lab.location ?? null,
		labs,
		station,
		detail,
		consent,
		records
	};
}
