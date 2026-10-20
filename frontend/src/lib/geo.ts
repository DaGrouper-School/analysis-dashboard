import type { GeoPoint } from '#lib/domain/types.ts';

/** Parses text inputs into a point; invalid input becomes NaN so validation reports it. */
export function toGeoPoint(lat: string, lng: string): GeoPoint {
	const parse = (v: string) => (v.trim() === '' ? NaN : Number(v));
	return { lat: parse(lat), lng: parse(lng) };
}

/** Like `toGeoPoint`, but both fields blank means "no location". */
export function toOptionalGeoPoint(lat: string, lng: string): GeoPoint | null {
	return lat.trim() === '' && lng.trim() === '' ? null : toGeoPoint(lat, lng);
}

export function formatPoint({ lat, lng }: GeoPoint): string {
	return `${Math.abs(lat).toFixed(4)}° ${lat >= 0 ? 'N' : 'S'}, ${Math.abs(lng).toFixed(4)}° ${lng >= 0 ? 'E' : 'W'}`;
}
