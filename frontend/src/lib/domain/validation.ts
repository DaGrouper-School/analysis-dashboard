import type { Address, CreateLabInput, GeoPoint, ProfileInput, RegisterDeviceInput } from './types';

/** Normalises a phone number to E.164 (`+2519…`), or returns null when it is not valid. */
export function normalizePhone(raw: string): string | null {
	const trimmed = raw.trim();
	const digits = trimmed.replace(/[^\d]/g, '');
	if (!trimmed.startsWith('+') && !trimmed.startsWith('00')) return null;
	const national = trimmed.startsWith('00') ? digits.slice(2) : digits;
	const e164 = `+${national}`;
	return /^\+[1-9]\d{7,14}$/.test(e164) ? e164 : null;
}

export function isValidEmail(email: string): boolean {
	return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim());
}

export function validateProfile(input: ProfileInput): Record<string, string> {
	const errors: Record<string, string> = {};
	if (input.fullName.trim().length < 2) errors.fullName = 'Enter your full name.';
	if (!isValidEmail(input.email)) errors.email = 'Enter a valid email address.';
	return errors;
}

export function validateAddress(address: Address): Record<string, string> {
	const errors: Record<string, string> = {};
	if (!address.street.trim()) errors.street = 'Enter a street address.';
	if (!address.city.trim()) errors.city = 'Enter a city.';
	if (!address.country.trim()) errors.country = 'Enter a country.';
	return errors;
}

export function isValidOtp(code: string): boolean {
	return /^\d{6}$/.test(code);
}

export function isValidGeoPoint(point: GeoPoint): boolean {
	return (
		Number.isFinite(point.lat) &&
		Number.isFinite(point.lng) &&
		Math.abs(point.lat) <= 90 &&
		Math.abs(point.lng) <= 180
	);
}

export function validateLab(input: CreateLabInput): Record<string, string> {
	const errors: Record<string, string> = {};
	if (input.name.trim().length < 3) errors.name = 'Enter a lab name.';
	if (!input.placeName.trim()) errors.placeName = 'Enter where the lab is.';
	if (!input.country.trim()) errors.country = 'Enter a country.';
	if (!isValidGeoPoint(input.location)) errors.location = 'Enter a valid latitude and longitude.';
	if (input.siteProfile.description.trim().length < 10) {
		errors.description = 'Describe the site in a sentence or two.';
	}
	if (!input.siteProfile.researchQuestions.some((q) => q.trim())) {
		errors.researchQuestions = 'Add at least one research question.';
	}
	return errors;
}

export function validateDevice(input: RegisterDeviceInput): Record<string, string> {
	const errors: Record<string, string> = {};
	if (input.name.trim().length < 3) errors.name = 'Enter a device name.';
	if (!/^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$/.test(input.identifier.trim())) {
		errors.identifier = 'Use 3 to 64 letters, digits, dots, dashes or underscores.';
	}
	if (!input.placeName.trim()) errors.placeName = 'Enter where the device is.';
	if (!input.country.trim()) errors.country = 'Enter a country.';
	if (input.location && !isValidGeoPoint(input.location)) {
		errors.location = 'Enter a valid latitude and longitude, or leave both blank.';
	}
	return errors;
}

/** Great-circle distance in kilometres. */
export function distanceKm(a: GeoPoint, b: GeoPoint): number {
	const rad = Math.PI / 180;
	const dLat = (b.lat - a.lat) * rad;
	const dLng = (b.lng - a.lng) * rad;
	const h =
		Math.sin(dLat / 2) ** 2 +
		Math.cos(a.lat * rad) * Math.cos(b.lat * rad) * Math.sin(dLng / 2) ** 2;
	return 2 * 6371 * Math.asin(Math.sqrt(h));
}
