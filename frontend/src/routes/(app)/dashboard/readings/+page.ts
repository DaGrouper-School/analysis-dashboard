import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { sensorQuery } from '#lib/utils/sensor-selection.ts';
import type { PageLoad } from './$types';

/**
 * Raw measurements open in a dialog now, from a device or a value. An old link to this page goes
 * to the classroom's devices, where each one opens its raw data.
 */
export const load: PageLoad = ({ url }) => {
	const classroom = url.searchParams.get('classroom');
	const device = url.searchParams.get('source') ?? url.searchParams.get('device') ?? undefined;
	redirect(
		307,
		resolve('/(app)/dashboard/classrooms') +
			(classroom ? `${sensorQuery(classroom, device)}&tab=devices` : '?tab=devices')
	);
};
