import type { ConsentRequest, GuardianConsent, GuardianInput, User } from '#lib/domain/types.ts';
import { normalizeGuardianContact, validateGuardian } from '#lib/domain/consent.ts';
import {
	newId,
	readDb,
	updateDb,
	type MockDbState,
	type StoredConsent
} from '#lib/data/mock-db.ts';
import { ServiceError, type ConsentService, type ProfileService } from '../types';
import { currentUser } from './classrooms';

type Clock = { delay: () => Promise<void>; now: () => number };

function toGuardianConsent(stored: StoredConsent): GuardianConsent {
	return {
		status: stored.status,
		guardianName: stored.guardianName,
		guardianContact: stored.guardianContact,
		requestedAt: stored.requestedAt,
		decidedAt: stored.decidedAt,
		// Mock only, like the SMS code: the real backend sends the link and never returns it.
		devToken: stored.token
	};
}

function toRequest(state: MockDbState, stored: StoredConsent): ConsentRequest {
	const user = state.users.find((u) => u.id === stored.userId)!;
	const classroomIds = new Set(
		state.memberships.filter((m) => m.userId === user.id).map((m) => m.classroomId)
	);
	return {
		contributorName: user.fullName.split(/\s+/)[0] || 'Your child',
		guardianName: stored.guardianName,
		classroomNames: state.classrooms.filter((c) => classroomIds.has(c.id)).map((c) => c.name),
		status: stored.status,
		decidedAt: stored.decidedAt,
		coordinateDecimals: state.privacy.minorCoordinateDecimals
	};
}

/** The signed-in minor's side (AP-04): ask a guardian, and see where it stands. */
export function createMockGuardian({
	delay,
	now
}: Clock): Pick<ProfileService, 'requestGuardianConsent' | 'getGuardianConsent'> {
	return {
		async requestGuardianConsent(input: GuardianInput) {
			await delay();
			const errors = validateGuardian(input);
			if (Object.keys(errors).length) {
				throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
			}
			return updateDb((state) => {
				const user: User = currentUser(state);
				const at = new Date(now()).toISOString();
				// A new link replaces the old one, which stops working. Until the guardian answers,
				// the minor cannot send observations, whatever an earlier link said.
				state.guardianConsents = state.guardianConsents.filter((c) => c.userId !== user.id);
				const stored: StoredConsent = {
					userId: user.id,
					token: newId('consent'),
					guardianName: input.guardianName.trim(),
					guardianContact: normalizeGuardianContact(input.guardianContact),
					status: 'pending',
					requestedAt: at,
					decidedAt: null
				};
				state.guardianConsents.push(stored);
				user.minor = true;
				user.consent = 'pending';
				user.updatedAt = at;
				return toGuardianConsent(stored);
			});
		},

		async getGuardianConsent() {
			const state = readDb();
			const user = currentUser(state);
			const stored = state.guardianConsents.find((c) => c.userId === user.id);
			return stored ? toGuardianConsent(stored) : null;
		}
	};
}

/** The guardian's side: open the link, give or withdraw consent. No sign-in. */
export function createMockConsent({ delay, now }: Clock): ConsentService {
	function find(state: MockDbState, token: string) {
		const stored = state.guardianConsents.find((c) => c.token === token);
		if (!stored) {
			throw new ServiceError('not_found', 'This consent link is not valid any more.');
		}
		return stored;
	}

	return {
		async get(token) {
			await delay();
			const state = readDb();
			return toRequest(state, find(state, token));
		},

		async decide(token, give) {
			await delay();
			return updateDb((state) => {
				const stored = find(state, token);
				const at = new Date(now()).toISOString();
				const value = give ? 'given' : 'withdrawn';
				if (stored.status !== value) {
					stored.status = value;
					stored.decidedAt = at;
					const user = state.users.find((u) => u.id === stored.userId)!;
					user.consent = value;
					user.updatedAt = at;
					// Stand-in for the `consent` attribute assertion sent to SAM, which enforces it.
					state.samAssertions.push({
						userId: user.id,
						attribute: 'consent',
						value,
						assertedAt: at
					});
				}
				return toRequest(state, stored);
			});
		}
	};
}
