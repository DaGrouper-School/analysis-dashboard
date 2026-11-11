import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { filterClassrooms, readFilter } from '#lib/features/classroom/directory.ts';
import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

/**
 * The public Classrooms (AP-45) are one page: every classroom as a tab, one open at a time. This
 * address opens the first classroom the filter keeps, alphabetically, keeping the filter.
 */
export const load: PageLoad = async ({ url }) => {
	const { classrooms } = await services.publicSite.home();
	const placed = classrooms.filter((c) => c.location).sort((a, b) => a.name.localeCompare(b.name));
	const first = filterClassrooms(placed, readFilter(url.searchParams))[0] ?? placed[0];
	if (!first) return {};
	redirect(307, resolve('/classrooms/[classroom]', { classroom: first.id }) + url.search);
};
