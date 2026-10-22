import type { RawQuery, RawSortKey } from '#lib/domain/types.ts';
import { rememberClassroom } from '#lib/classroom-context.ts';
import { PAGE_SIZE, SORT_KEYS } from '#lib/raw-data.ts';
import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

const day = (value: string | null) =>
	value && /^\d{4}-\d{2}-\d{2}$/.test(value) ? value : undefined;

/**
 * Every raw value of the classroom in context (AP-14), filtered and sorted by SAM's export
 * service. Filters live in the address; `device` from the dashboard picks that source.
 */
export const load: PageLoad = async ({ parent, url }) => {
	const { contextClassroom } = await parent();
	const parameters = await services.registry.listParameters();
	if (!contextClassroom) return { query: null, page: null, parameters, limit: PAGE_SIZE };
	rememberClassroom(contextClassroom.id);
	const get = (key: string) => url.searchParams.get(key) || undefined;
	const sort = get('sort');
	const query: RawQuery = {
		classroomId: contextClassroom.id,
		from: day(url.searchParams.get('from')),
		to: day(url.searchParams.get('to')),
		parameterId: get('param'),
		sourceId: get('source') ?? get('device'),
		labId: get('site'),
		sort: SORT_KEYS.includes(sort as RawSortKey) ? (sort as RawSortKey) : undefined,
		order: get('order') === 'asc' ? 'asc' : get('order') === 'desc' ? 'desc' : undefined
	};
	const limit = Math.max(PAGE_SIZE, Number(url.searchParams.get('n')) || PAGE_SIZE);
	return { query, page: await services.exports.rawValues(query, limit), parameters, limit };
};
