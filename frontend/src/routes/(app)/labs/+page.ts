import { resolve } from '$app/paths';
import { redirect } from '@sveltejs/kit';
import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/**
 * Labs of the classroom in context. With exactly one lab, it opens straight away; new labs are
 * added from the classroom's page.
 */
export const load: PageLoad = async ({ parent }) => {
	const { contextClassroom } = await parent();
	const labs = contextClassroom ? await services.labs.listForClassroom(contextClassroom.id) : [];
	if (labs.length === 1) redirect(307, resolve('/(app)/labs/[labId]', { labId: labs[0].id }));
	return {
		labs,
		canAddLabs:
			contextClassroom?.myRole === 'institution_head' || contextClassroom?.myRole === 'facilitator'
	};
};
