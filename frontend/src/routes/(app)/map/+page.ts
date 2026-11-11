import { sensorQuery } from '#lib/sensor-selection.ts';
import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

/** The map gave way to Classrooms in the nav. Old links open the classroom they named. */
export const load: PageLoad = ({ url }) => {
	const classroom = url.searchParams.get('classroom');
	redirect(
		307,
		classroom ? resolve('/(app)/dashboard') + sensorQuery(classroom) : resolve('/(app)/dashboard')
	);
};
