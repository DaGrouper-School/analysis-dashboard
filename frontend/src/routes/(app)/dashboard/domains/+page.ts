import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { services } from '#lib/services/index.ts';
import { selectSensor, sensorQuery } from '#lib/utils/sensor-selection.ts';
import type { PageLoad } from './$types';

/**
 * The Domains page: what one of the user's classrooms measures, device by device. Domain details
 * are for a classroom's members only (researchers and up): another classroom goes back to its
 * overview, which shows its public data.
 */
export const load: PageLoad = async ({ url, depends }) => {
	depends('app:classroom');
	const asked = url.searchParams.get('classroom');
	const selection = await selectSensor(url);
	if (asked && !selection.classrooms.some((c) => c.id === asked)) {
		redirect(307, resolve('/(app)/dashboard') + sensorQuery(asked));
	}
	const station = selection.classroom
		? await services.publicSite.station(selection.classroom.id)
		: null;
	return { ...selection, station };
};
