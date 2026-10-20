import type { Address, ProfileInput } from './types';

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
