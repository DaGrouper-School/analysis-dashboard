import type { ClassroomSetupStatus, InstitutionSetup, User } from '#lib/domain/types.ts';
import {
	answersFrom,
	infoFrom,
	validateInstitutionAnswers
} from '#lib/domain/institution-setup.ts';
import { readDb, updateDb, type MockDbState, type StoredClassroom } from '#lib/data/mock-db.ts';
import { ServiceError, type ClassroomService } from '../types';
import { currentUser, visibleClassroom } from './classrooms';

type SetupService = Pick<
	ClassroomService,
	'institutionSetupForm' | 'institutionSetup' | 'saveInstitutionSetup' | 'setHostAgreement'
>;

/** Where a classroom's setup is, for its head and platform admins; null for everyone else. */
export function setupStatus(
	classroom: StoredClassroom,
	can: { setUpInstitution: boolean; markHostAgreement: boolean }
): ClassroomSetupStatus | null {
	if (!can.setUpInstitution && !can.markHostAgreement) return null;
	return {
		institutionDone: classroom.institutionSetup.completedAt !== null,
		hostAgreementSigned: classroom.profile.hostAgreementSigned
	};
}

function toSetup(classroom: StoredClassroom): InstitutionSetup {
	return {
		classroomId: classroom.id,
		classroomName: classroom.name,
		institutionName: classroom.institutionName,
		answers: answersFrom(classroom.profile.institution, classroom.profile.ecosystems),
		step: classroom.institutionSetup.step,
		completedAt: classroom.institutionSetup.completedAt
	};
}

function headsClassroom(state: MockDbState, user: User, classroomId: string) {
	const { classroom, can } = visibleClassroom(state, user, classroomId);
	if (!can.setUpInstitution) {
		throw new ServiceError('forbidden', 'Only the institution head can set up the institution.');
	}
	return classroom;
}

/** The institution setup (AP-40), part of the classrooms service. */
export function createMockInstitutionSetup({
	delay,
	now
}: {
	delay: () => Promise<void>;
	now: () => number;
}): SetupService {
	return {
		async institutionSetupForm() {
			await delay();
			return structuredClone(readDb().institutionSetupForm);
		},

		async institutionSetup(classroomId) {
			await delay();
			const state = readDb();
			return toSetup(headsClassroom(state, currentUser(state), classroomId));
		},

		async saveInstitutionSetup(classroomId, input) {
			await delay();
			return updateDb((state) => {
				const classroom = headsClassroom(state, currentUser(state), classroomId);
				const form = state.institutionSetupForm;
				// Unfinished answers save; whatever is answered must still be valid.
				const errors = validateInstitutionAnswers(input.answers, form, {
					now: now(),
					draft: !input.finish
				});
				const fields = Object.keys(errors);
				if (fields.length) {
					throw new ServiceError(
						'invalid_input',
						errors[fields[0] as keyof typeof errors]!,
						errors as Record<string, string>
					);
				}
				const { info, ecosystems } = infoFrom(input.answers);
				classroom.profile.institution = info;
				classroom.profile.ecosystems = ecosystems;
				const last = form.steps.length; // The review step, after the questions.
				classroom.institutionSetup.step = Math.min(Math.max(0, input.step), last);
				if (input.finish && !classroom.institutionSetup.completedAt) {
					classroom.institutionSetup.completedAt = new Date(now()).toISOString();
				}
				return toSetup(classroom);
			});
		},

		async setHostAgreement(classroomId, signed) {
			await delay();
			return updateDb((state) => {
				const { classroom, can } = visibleClassroom(state, currentUser(state), classroomId);
				if (!can.markHostAgreement) {
					throw new ServiceError(
						'forbidden',
						'Only platform admins can mark the host agreement signed.'
					);
				}
				classroom.profile.hostAgreementSigned = signed;
				return setupStatus(classroom, can)!;
			});
		}
	};
}
