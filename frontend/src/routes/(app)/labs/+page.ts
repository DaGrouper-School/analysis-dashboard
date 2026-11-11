import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { sectionQuery } from '#lib/sensor-selection.ts';
import type { PageLoad } from './$types';

/** A classroom's labs are its dashboard Labs tab (AP-45). */
export const load: PageLoad = async ({ parent }) => {
	const { contextClassroom } = await parent();
	redirect(
		307,
		contextClassroom
			? resolve('/(app)/dashboard') + sectionQuery(contextClassroom.id, 'labs')
			: resolve('/(app)/dashboard')
	);
};
