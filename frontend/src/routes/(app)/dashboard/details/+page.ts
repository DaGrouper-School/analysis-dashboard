import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { sensorQuery } from '#lib/utils/sensor-selection.ts';
import { loadDashboard } from '../+page.ts';
import type { PageLoad } from './$types';

/**
 * The classroom details page: the dashboard's data, its sections as tabs. Members only: a
 * classroom the user is only visiting stays on its overview, which shows its public data.
 */
export const load: PageLoad = async (event) => {
	const data = await loadDashboard(event);
	if (data.visiting) {
		redirect(307, resolve('/(app)/dashboard') + sensorQuery(data.visiting.classroom.id));
	}
	return data;
};
