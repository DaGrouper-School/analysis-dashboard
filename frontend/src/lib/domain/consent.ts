import type {
	ConsentSheetContent,
	ConsentStatus,
	GuardianInput,
	PaperConsentInput,
	User
} from '#lib/domain/types.ts';
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

/**
 * A paper consent sheet as recorded (AP-38): the guardian's name, and the day on the sheet, which
 * is a real date and not later than `today` (YYYY-MM-DD, in the recorder's time zone).
 */
export function validatePaperConsent(
	input: PaperConsentInput,
	today: string
): Record<string, string> {
	const errors: Record<string, string> = {};
	if (!input.guardianName.trim()) errors.guardianName = 'Enter the guardian’s name from the sheet.';
	const day = input.signedOn.trim();
	const parsed = /^\d{4}-\d{2}-\d{2}$/.test(day) ? new Date(`${day}T00:00:00Z`) : null;
	if (!day) errors.signedOn = 'Enter the date on the sheet.';
	else if (!parsed || Number.isNaN(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== day) {
		errors.signedOn = 'Enter a valid date.';
	} else if (day > today) errors.signedOn = 'The date on the sheet can’t be in the future.';
	return errors;
}

/** Today as YYYY-MM-DD in the local time zone, for date fields and checks. */
export function localDay(at: Date = new Date()): string {
	const pad = (n: number) => String(n).padStart(2, '0');
	return `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}`;
}

/** The sheet's words with one classroom's details in place of `{classroom}` and the others. */
export function fillConsentSheet(
	content: ConsentSheetContent,
	values: { classroom: string; institution: string; decimals: number }
): ConsentSheetContent {
	const fill = (text: string) =>
		text
			.replaceAll('{classroom}', values.classroom)
			.replaceAll('{institution}', values.institution)
			.replaceAll('{decimals}', String(values.decimals));
	return {
		title: fill(content.title),
		intro: fill(content.intro),
		sections: content.sections.map((s) => ({ heading: fill(s.heading), body: fill(s.body) })),
		choices: { give: fill(content.choices.give), withdraw: fill(content.choices.withdraw) },
		contributorField: fill(content.contributorField),
		fields: content.fields.map(fill),
		officeNote: fill(content.officeNote)
	};
}
