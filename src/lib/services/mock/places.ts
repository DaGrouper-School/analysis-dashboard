import type { Address } from '#lib/domain/types.ts';
import { ServiceError, type PlacesService } from '../types';

/** A small offline stand-in for Google Places autocomplete, covering the pilot regions. */
const PLACES: (Address & { placeId: string })[] = [
	{
		placeId: 'pl_shashamane',
		street: 'Kebele 04',
		city: 'Shashamane',
		state: 'Oromia',
		zip: '',
		country: 'Ethiopia',
		lat: 7.2,
		lng: 38.6
	},
	{
		placeId: 'pl_addis',
		street: 'Churchill Avenue',
		city: 'Addis Ababa',
		state: 'Addis Ababa',
		zip: '1000',
		country: 'Ethiopia',
		lat: 9.03,
		lng: 38.74
	},
	{
		placeId: 'pl_hawassa',
		street: 'Lake Road',
		city: 'Hawassa',
		state: 'Sidama',
		zip: '',
		country: 'Ethiopia',
		lat: 7.06,
		lng: 38.48
	},
	{
		placeId: 'pl_nassau',
		street: '14 Bay Street',
		city: 'Nassau',
		state: 'New Providence',
		zip: '',
		country: 'Bahamas',
		lat: 25.06,
		lng: -77.34
	},
	{
		placeId: 'pl_freeport',
		street: 'East Mall Drive',
		city: 'Freeport',
		state: 'Grand Bahama',
		zip: '',
		country: 'Bahamas',
		lat: 26.53,
		lng: -78.7
	},
	{
		placeId: 'pl_hargeisa',
		street: 'Independence Road',
		city: 'Hargeisa',
		state: 'Maroodi Jeex',
		zip: '',
		country: 'Somalia',
		lat: 9.56,
		lng: 44.06
	},
	{
		placeId: 'pl_mogadishu',
		street: 'Maka Al Mukarama Road',
		city: 'Mogadishu',
		state: 'Banaadir',
		zip: '',
		country: 'Somalia',
		lat: 2.05,
		lng: 45.32
	},
	{
		placeId: 'pl_kingston',
		street: 'King Street',
		city: 'Kingston',
		state: 'Kingston Parish',
		zip: '',
		country: 'Jamaica',
		lat: 17.97,
		lng: -76.79
	}
];

export function createMockPlaces(delay: () => Promise<void>): PlacesService {
	return {
		async autocomplete(query) {
			const q = query.trim().toLowerCase();
			if (q.length < 2) return [];
			await delay();
			return PLACES.filter((p) =>
				[p.street, p.city, p.state, p.country].some((part) => part.toLowerCase().includes(q))
			).map((p) => ({
				placeId: p.placeId,
				primaryText: p.street,
				secondaryText: [p.city, p.state, p.country].filter(Boolean).join(', ')
			}));
		},

		async getAddress(placeId) {
			const place = PLACES.find((p) => p.placeId === placeId);
			if (!place) throw new ServiceError('not_found', 'That place could not be found.');
			return { ...place };
		}
	};
}
