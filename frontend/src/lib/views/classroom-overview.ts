import type { InstitutionOverview, PublicStation } from '#lib/domain/types.ts';

/**
 * The institution card of a classroom's overview: always a card, with just the classroom's name
 * until there is more, so the row never reflows.
 */
export function institutionCard(station: PublicStation): InstitutionOverview {
	const { classroom } = station;
	return (
		station.institution ?? {
			name: classroom.name,
			location: classroom.location,
			deviceTypes: [],
			photoUrl: null,
			photoAlt: '',
			history: '',
			hub: null,
			website: null
		}
	);
}
