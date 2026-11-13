import type {
	InstitutionAnswers,
	InstitutionField,
	InstitutionInfo,
	InstitutionQuestion,
	InstitutionSetupForm
} from './types';
import { validateInstitutionPhoto } from './validation';

/**
 * The institution setup (AP-40): what an institution head answers about their institution, one
 * step at a time. The rules the backend checks too.
 */

/** The answers already given, from the classroom's overview and profile. */
export function answersFrom(info: InstitutionInfo, ecosystems: string[]): InstitutionAnswers {
	return {
		photo: info.photoUrl,
		photoAlt: info.photoAlt,
		foundedYear: info.foundedYear === null ? '' : String(info.foundedYear),
		history: info.history,
		ecosystems: [...ecosystems],
		hub: info.hub ?? '',
		website: info.website ?? '',
		showPublicly: info.showPublicly === null ? '' : info.showPublicly ? 'yes' : 'no'
	};
}

/** The overview and ecosystems the answers describe, trimmed; blanks become nothing. */
export function infoFrom(answers: InstitutionAnswers): {
	info: InstitutionInfo;
	ecosystems: string[];
} {
	const year = answers.foundedYear.trim();
	return {
		info: {
			photoUrl: answers.photo,
			photoAlt: answers.photo ? answers.photoAlt.trim() : '',
			history: answers.history.trim(),
			hub: answers.hub.trim() || null,
			website: answers.website.trim() || null,
			foundedYear: year ? Number(year) : null,
			showPublicly: answers.showPublicly ? answers.showPublicly === 'yes' : null
		},
		ecosystems: [...new Set(answers.ecosystems.map((e) => e.trim()).filter(Boolean))]
	};
}

function isBlank(answers: InstitutionAnswers, field: InstitutionField): boolean {
	const value = answers[field];
	if (value === null) return true;
	if (Array.isArray(value)) return !value.some((v) => v.trim());
	return !value.trim();
}

function check(
	q: InstitutionQuestion,
	answers: InstitutionAnswers,
	thisYear: number,
	requireAnswers: boolean
): string | null {
	if (isBlank(answers, q.field)) {
		if (!q.required || !requireAnswers) return null;
		return q.kind === 'photo'
			? 'Choose a photo.'
			: q.kind === 'multi_choice' || q.kind === 'yes_no'
				? 'Choose an answer.'
				: 'Answer this question.';
	}
	const value = answers[q.field];
	switch (q.kind) {
		case 'photo':
			return validateInstitutionPhoto({ image: answers.photo, alt: 'set' }).image ?? null;
		case 'year': {
			const year = Number(String(value).trim());
			return Number.isInteger(year) && year >= 1000 && year <= thisYear
				? null
				: `Enter a year, up to ${thisYear}.`;
		}
		case 'url':
			return /^https?:\/\/\S+\.\S+$/.test(String(value).trim())
				? null
				: 'Enter a web address starting with https://, or leave it empty.';
		case 'yes_no':
			return value === 'yes' || value === 'no' ? null : 'Choose yes or no.';
		case 'multi_choice':
			return null;
		default: {
			const text = String(value).trim();
			if (q.maxLength && text.length > q.maxLength) {
				return `Keep it to ${q.maxLength} characters.`;
			}
			return null;
		}
	}
}

/**
 * Errors by field, for the questions of one step (by id) or all of them. Optional questions are
 * checked only when answered; with `draft`, required ones too, so unfinished answers save.
 */
export function validateInstitutionAnswers(
	answers: InstitutionAnswers,
	form: InstitutionSetupForm,
	options: { step?: string; now?: number; draft?: boolean } = {}
): Partial<Record<InstitutionField, string>> {
	const thisYear = new Date(options.now ?? Date.now()).getUTCFullYear();
	const errors: Partial<Record<InstitutionField, string>> = {};
	for (const q of form.questions) {
		if (options.step && q.step !== options.step) continue;
		const error = check(q, answers, thisYear, !options.draft);
		if (error) errors[q.field] = error;
	}
	return errors;
}

/** The step a field is asked on, counted from 0, for sending the head back to it. */
export function stepOfField(form: InstitutionSetupForm, field: string): number {
	const question = form.questions.find((q) => q.field === field);
	return Math.max(
		0,
		form.steps.findIndex((s) => s.id === question?.step)
	);
}
