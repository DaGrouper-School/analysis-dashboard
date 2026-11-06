import type { ConsentStatus, GuardianInput, User } from '#lib/domain/types.ts';
import { isValidEmail, normalizePhone } from '#lib/domain/validation.ts';

/**
 * Why the user may not send observations yet (AP-04), or null when they may. Only minors without
 * a guardian's consent are held back.
 */
export function contributionBlock(user: Pick<User, 'minor' | 'consent'>): string | null {
	if (!user.minor || user.consent === 'given') return null;
	return user.consent === 'withdrawn'
		? 'Your guardian withdrew their consent, so you cannot send observations. Ask them to give it again.'
		: 'Your guardian needs to give consent before you can send observations.';
}

/** What a consent status means, in a word or two. */
export const CONSENT_LABELS: Record<ConsentStatus, string> = {
	not_needed: 'Not needed',
	pending: 'Waiting for guardian',
	given: 'Given',
	withdrawn: 'Withdrawn'
};

/** The guardian's name, and an email address or phone number to reach them. */
export function validateGuardian(input: GuardianInput): Record<string, string> {
	const errors: Record<string, string> = {};
	if (!input.guardianName.trim()) errors.guardianName = 'Enter your guardian’s name.';
	const contact = input.guardianContact.trim();
	if (!contact) errors.guardianContact = 'Enter their email address or phone number.';
	else if (!isValidEmail(contact) && !normalizePhone(contact)) {
		errors.guardianContact = 'Enter a valid email address, or a phone number with country code.';
	}
	return errors;
}

/** The contact as stored: a lower-case email, or a phone number in E.164. */
export function normalizeGuardianContact(contact: string): string {
	const trimmed = contact.trim();
	return isValidEmail(trimmed) ? trimmed.toLowerCase() : (normalizePhone(trimmed) ?? trimmed);
}
