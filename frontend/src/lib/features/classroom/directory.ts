import type { PublicClassroom, SiteContent } from '#lib/domain/types.ts';

/** The public Classrooms filters (AP-28, AP-45); an empty value means any. */
export interface DirectoryFilter {
	region: string;
	country: string;
	stage: string;
}

export const NO_FILTER: DirectoryFilter = { region: '', country: '', stage: '' };

/** What a filter reads from a classroom: a public classroom, or a place on its page. */
type Filterable = Pick<PublicClassroom, 'regionId' | 'country' | 'stageId'>;

export function filterClassrooms<T extends Filterable>(
	classrooms: T[],
	filter: DirectoryFilter
): T[] {
	return classrooms.filter(
		(c) =>
			(!filter.region || c.regionId === filter.region) &&
			(!filter.country || c.country === filter.country) &&
			(!filter.stage || c.stageId === filter.stage)
	);
}

/** The choices each filter offers, from the classrooms themselves. */
export function filterOptions(classrooms: Filterable[], content: SiteContent) {
	const used = (values: (string | null)[]) => new Set(values.filter((v): v is string => !!v));
	const regions = used(classrooms.map((c) => c.regionId));
	const stages = used(classrooms.map((c) => c.stageId));
	return {
		regions: content.regions.filter((r) => regions.has(r.id)),
		countries: [...used(classrooms.map((c) => c.country))].sort((a, b) => a.localeCompare(b)),
		stages: content.stages.filter((s) => stages.has(s.id))
	};
}

/** The filter as kept in the page's URL, so a filtered directory can be shared. */
export function readFilter(params: URLSearchParams): DirectoryFilter {
	return {
		region: params.get('region') ?? '',
		country: params.get('country') ?? '',
		stage: params.get('stage') ?? ''
	};
}

export function filterQuery(filter: DirectoryFilter): string {
	const parts = (Object.keys(filter) as (keyof DirectoryFilter)[])
		.filter((k) => filter[k])
		.map((k) => `${k}=${encodeURIComponent(filter[k])}`);
	return parts.length ? `?${parts.join('&')}` : '';
}
