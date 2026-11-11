import type { GeoPoint } from '#lib/domain/types.ts';

/** Parses text inputs into a point; invalid input becomes NaN so validation reports it. */
export function toGeoPoint(lat: string, lng: string): GeoPoint {
	const parse = (v: string) => (v.trim() === '' ? NaN : Number(v));
	return { lat: parse(lat), lng: parse(lng) };
}

export function formatPoint({ lat, lng }: GeoPoint): string {
	return `${Math.abs(lat).toFixed(4)}° ${lat >= 0 ? 'N' : 'S'}, ${Math.abs(lng).toFixed(4)}° ${lng >= 0 ? 'E' : 'W'}`;
}

/** A rounded position as "9.03° N, 38.74° E". */
export function formatCoordinates(location: { lat: number; lng: number }): string {
	const part = (value: number, positive: string, negative: string) =>
		`${Math.abs(value).toFixed(2)}° ${value >= 0 ? positive : negative}`;
	return `${part(location.lat, 'N', 'S')}, ${part(location.lng, 'E', 'W')}`;
}
