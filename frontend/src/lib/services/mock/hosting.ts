import { validateHostApplication } from '#lib/domain/hosting.ts';
import type { HostApplication, HostApplicationInput } from '#lib/domain/types.ts';
import { normalizePhone } from '#lib/domain/validation.ts';
import { newId, readDb, updateDb, type StoredHostApplication } from '#lib/data/mock-db.ts';
import { ServiceError, type HostingService } from '../types';
import { currentUser } from './classrooms';

type Clock = { delay: () => Promise<void>; now: () => number };

function toApplication(stored: StoredHostApplication): HostApplication {
	const { token: _token, ...application } = structuredClone(stored);
	void _token;
	return application;
}

/** Keeps only what the form asks: answers to the configured questions, trimmed. */
function clean(input: HostApplicationInput, questionIds: string[]): HostApplicationInput {
	const phone = input.contactPhone.trim();
	return {
		institutionName: input.institutionName.trim(),
		contactName: input.contactName.trim(),
		contactRole: input.contactRole.trim(),
		contactEmail: input.contactEmail.trim().toLowerCase(),
		contactPhone: phone ? (normalizePhone(phone) ?? phone) : '',
		answers: Object.fromEntries(
			questionIds
				.map((id) => [id, input.answers[id]?.trim() ?? ''] as const)
				.filter(([, answer]) => answer)
		),
		site: {
			placeName: input.site.placeName.trim(),
			country: input.site.country.trim(),
			ecosystems: input.site.ecosystems.map((e) => e.trim()).filter(Boolean),
			learners: input.site.learners.trim(),
			studyGoals: input.site.studyGoals.trim()
		}
	};
}

/** Applying to host a classroom (AP-30): public to apply, platform admins to decide. */
export function createMockHosting({ delay, now }: Clock): HostingService {
	const admin = (state: Parameters<typeof currentUser>[0]) => {
		const user = currentUser(state);
		if (!user.platformAdmin) {
			throw new ServiceError('forbidden', 'Only platform admins review applications.');
		}
		return user;
	};

	return {
		async questions() {
			await delay();
			return structuredClone(readDb().hostQuestions);
		},

		async apply(input) {
			await delay();
			return updateDb((state) => {
				const errors = validateHostApplication(input, state.hostQuestions);
				if (Object.keys(errors).length) {
					throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
				}
				const stored: StoredHostApplication = {
					id: newId('happ'),
					token: newId('host'),
					...clean(
						input,
						state.hostQuestions.map((q) => q.id)
					),
					status: 'submitted',
					submittedAt: new Date(now()).toISOString(),
					decidedAt: null,
					classroomId: null
				};
				state.hostApplications.push(stored);
				// Mock only: the real backend emails the status link and never returns the token.
				return { application: toApplication(stored), token: stored.token };
			});
		},

		async status(token) {
			await delay();
			const stored = readDb().hostApplications.find((a) => a.token === token);
			if (!stored) throw new ServiceError('not_found', 'This status link is not valid.');
			return toApplication(stored);
		},

		async list() {
			await delay();
			const state = readDb();
			admin(state);
			return state.hostApplications
				.map(toApplication)
				.sort((a, b) => b.submittedAt.localeCompare(a.submittedAt));
		},

		async decide(id, accept) {
			await delay();
			return updateDb((state) => {
				admin(state);
				const stored = state.hostApplications.find((a) => a.id === id);
				if (!stored) throw new ServiceError('not_found', 'Application not found.');
				if (stored.status !== 'submitted') {
					throw new ServiceError('conflict', 'This application has already been decided.');
				}
				stored.status = accept ? 'accepted' : 'declined';
				stored.decidedAt = new Date(now()).toISOString();
				return toApplication(stored);
			});
		}
	};
}
