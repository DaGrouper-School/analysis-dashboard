import type { RawQuery, RawSortKey } from '#lib/domain/types.ts';
import { rememberClassroom } from '#lib/utils/classroom-context.ts';
import { PAGE_SIZE, SORT_KEYS } from '#lib/features/measurements/raw-data.ts';
import { services } from '#lib/services/index.ts';
import { sourceLooks, type SourceLooks } from '#lib/features/measurements/source-looks.ts';
import type { PageLoad } from './$types';

const day = (value: string | null) =>
	value && /^\d{4}-\d{2}-\d{2}$/.test(value) ? value : undefined;

/**
 * Every raw value of the classroom in context (AP-14), filtered and sorted by SAM's export
 * service. Filters and the page live in the address; `device` from the dashboard picks that source.
 */
export const load: PageLoad = async ({ parent, url }) => {
	const { contextClassroom, deviceTypes } = await parent();
	const parameters = await services.registry.listParameters();
	if (!contextClassroom) return { query: null, page: null, parameters, looks: {} as SourceLooks };
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
	const pageNo = Math.max(1, Math.floor(Number(url.searchParams.get('page'))) || 1);
	const [page, devices] = await Promise.all([
		services.exports.rawValues(query, { page: pageNo, pageSize: PAGE_SIZE }),
		services.labs.listClassroomDevices(contextClassroom.id)
	]);
	// Each device's registry icon and colour, for the Measurement dropdown, like the picker's.
	const looks = sourceLooks(devices, deviceTypes);
	return { query, page, parameters, looks };
};
