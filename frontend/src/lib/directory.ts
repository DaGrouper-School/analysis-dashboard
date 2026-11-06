import type { PublicClassroom, SiteContent } from '#lib/domain/types.ts';

/** The directory's filters (AP-28); an empty value means any. */
export interface DirectoryFilter {
	region: string;
	country: string;
	stage: string;
}

export const NO_FILTER: DirectoryFilter = { region: '', country: '', stage: '' };

export function filterClassrooms(
	classrooms: PublicClassroom[],
	filter: DirectoryFilter
): PublicClassroom[] {
	return classrooms.filter(
		(c) =>
			(!filter.region || c.regionId === filter.region) &&
			(!filter.country || c.country === filter.country) &&
			(!filter.stage || c.stageId === filter.stage)
	);
}

export interface DirectoryGroup {
	/** Null for classrooms with no region yet. */
	region: { id: string; label: string } | null;
	countries: { country: string | null; classrooms: PublicClassroom[] }[];
}

/**
 * Classrooms by region, in the site content's order, then by country and name. Classrooms
 * with no region, or no country, come last in their list.
 */
export function groupClassrooms(
	classrooms: PublicClassroom[],
	regions: SiteContent['regions']
): DirectoryGroup[] {
	const known = new Set(regions.map((r) => r.id));
	const groups = [
		...regions.map((region) => ({
			region,
			members: classrooms.filter((c) => c.regionId === region.id)
		})),
		{ region: null, members: classrooms.filter((c) => !c.regionId || !known.has(c.regionId)) }
	];
	return groups
		.filter((g) => g.members.length)
		.map(({ region, members }) => {
			const countries = [...new Set(members.map((c) => c.country))].sort((a, b) =>
				a === null ? 1 : b === null ? -1 : a.localeCompare(b)
			);
			return {
				region,
				countries: countries.map((country) => ({
					country,
					classrooms: members
						.filter((c) => c.country === country)
						.sort((a, b) => a.name.localeCompare(b.name))
				}))
			};
		});
}

/** The choices each filter offers, from the classrooms themselves. */
export function filterOptions(classrooms: PublicClassroom[], content: SiteContent) {
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
