import type {
	HostApplicationInput,
	HostApplicationStatus,
	HostQuestion
} from '#lib/domain/types.ts';
import { isValidEmail, normalizePhone } from '#lib/domain/validation.ts';

/** Applying to host a Global Classroom (AP-30): the rules the backend checks too. */

export const HOST_STATUS_LABELS: Record<HostApplicationStatus, string> = {
	submitted: 'Submitted, waiting for review',
	accepted: 'Accepted',
	declined: 'Not accepted'
};

/** Which step of the form a field is on, for sending the applicant back to it. */
export type HostStep = 'institution' | 'readiness' | 'site';

export function stepOfField(field: string): HostStep {
	if (field.startsWith('answers.')) return 'readiness';
	if (field.startsWith('site.')) return 'site';
	return 'institution';
}

export function emptyHostApplication(): HostApplicationInput {
	return {
		institutionName: '',
		contactName: '',
		contactRole: '',
		contactEmail: '',
		contactPhone: '',
		answers: {},
		site: { placeName: '', country: '', ecosystems: [], learners: '', studyGoals: '' }
	};
}

/** Field errors by field: `contactEmail`, `answers.<question>`, `site.placeName`, … */
export function validateHostApplication(
	input: HostApplicationInput,
	questions: HostQuestion[],
	only?: HostStep
): Record<string, string> {
	const errors: Record<string, string> = {};
	const on = (step: HostStep) => !only || only === step;
	const blank = (v: string | undefined) => !v || !v.trim();

	if (on('institution')) {
		if (blank(input.institutionName)) errors.institutionName = 'Enter the institution’s name.';
		if (blank(input.contactName)) errors.contactName = 'Enter your name.';
		if (blank(input.contactRole)) errors.contactRole = 'Enter your role at the institution.';
		if (blank(input.contactEmail)) errors.contactEmail = 'Enter your email.';
		else if (!isValidEmail(input.contactEmail.trim())) {
			errors.contactEmail = 'Enter a valid email address.';
		}
		if (!blank(input.contactPhone) && !normalizePhone(input.contactPhone)) {
			errors.contactPhone = 'Enter a valid phone number, or leave it empty.';
		}
	}

	if (on('readiness')) {
		for (const q of questions) {
			const answer = input.answers[q.id]?.trim() ?? '';
			const key = `answers.${q.id}`;
			if (!answer) {
				if (q.required) errors[key] = 'Answer this question.';
			} else if (q.kind === 'yes_no' && answer !== 'yes' && answer !== 'no') {
				errors[key] = 'Choose yes or no.';
			} else if (q.kind === 'number' && !(Number(answer) >= 0 && Number.isFinite(Number(answer)))) {
				errors[key] = 'Enter a number.';
			}
		}
	}

	if (on('site')) {
		if (blank(input.site.placeName)) errors['site.placeName'] = 'Enter the town or area.';
		if (blank(input.site.country)) errors['site.country'] = 'Enter the country.';
		if (!input.site.ecosystems.some((e) => e.trim())) {
			errors['site.ecosystems'] = 'Name at least one ecosystem.';
		}
		if (blank(input.site.learners)) errors['site.learners'] = 'Say who the learners would be.';
		if (blank(input.site.studyGoals)) {
			errors['site.studyGoals'] = 'Say what the classroom wants to study.';
		}
	}
	return errors;
}

/** Splits "reef, mangrove; school garden" into its ecosystems. */
export function parseEcosystems(text: string): string[] {
	return text
		.split(/[,;\n]/)
		.map((e) => e.trim())
		.filter(Boolean);
}
