import type { Address, AuthResult, PhoneChallenge, User } from '#lib/domain/types.ts';
import {
	isValidOtp,
	normalizePhone,
	validateAddress,
	validateProfile
} from '#lib/domain/validation.ts';
import {
	newId,
	readDb,
	updateDb,
	type MockDbState,
	type StoredChallenge
} from '#lib/data/mock-db.ts';
import { ServiceError, type Services } from '../types';
import { createMockClassrooms } from './classrooms';
import { createMockLabs } from './labs';
import { createMockPlaces } from './places';
import { createMockRegistry } from './registry';

const RESEND_AFTER_MS = 30_000;
const CODE_TTL_MS = 5 * 60_000;
const MAX_ATTEMPTS = 5;

export interface MockOptions {
	/** Simulated network latency in ms. */
	latency?: number;
	now?: () => number;
}

/** A localStorage-backed implementation of every service, behaving like the future backend. */
export function createMockServices({ latency = 250, now = Date.now }: MockOptions = {}): Services {
	const delay = () => new Promise<void>((r) => setTimeout(r, latency));
	const iso = (ms: number) => new Date(ms).toISOString();

	function currentUser(state: MockDbState): User {
		const user = state.session && state.users.find((u) => u.id === state.session!.userId);
		if (!user) throw new ServiceError('unauthenticated', 'Please sign in again.');
		return user;
	}

	function startSession(state: MockDbState, userId: string) {
		state.session = { token: newId('sess'), userId };
	}

	function touch(user: User) {
		user.updatedAt = iso(now());
	}

	function issueChallenge(
		state: MockDbState,
		rawPhone: string,
		purpose: StoredChallenge['purpose'],
		userId: string | null
	): PhoneChallenge {
		const phone = normalizePhone(rawPhone);
		if (!phone) {
			throw new ServiceError('invalid_input', 'Enter a valid phone number with country code.', {
				phone: 'Enter a valid phone number with country code.'
			});
		}
		const t = now();
		const previous = state.challenges.find((c) => c.phone === phone && c.purpose === purpose);
		if (previous && Date.parse(previous.resendAvailableAt) > t) {
			throw new ServiceError('rate_limited', 'Please wait before requesting another code.');
		}
		state.challenges = state.challenges.filter(
			(c) => c !== previous && Date.parse(c.expiresAt) > t
		);
		const challenge: StoredChallenge = {
			requestId: newId('otp'),
			phone,
			code: String(Math.floor(Math.random() * 1_000_000)).padStart(6, '0'),
			purpose,
			userId,
			attempts: 0,
			resendAvailableAt: iso(t + RESEND_AFTER_MS),
			expiresAt: iso(t + CODE_TTL_MS)
		};
		state.challenges.push(challenge);
		return {
			requestId: challenge.requestId,
			phone,
			resendAvailableAt: challenge.resendAvailableAt,
			expiresAt: challenge.expiresAt,
			devCode: challenge.code
		};
	}

	function consumeChallenge(
		state: MockDbState,
		requestId: string,
		code: string,
		purpose: StoredChallenge['purpose']
	): StoredChallenge {
		const challenge = state.challenges.find(
			(c) => c.requestId === requestId && c.purpose === purpose
		);
		if (!challenge || Date.parse(challenge.expiresAt) <= now()) {
			throw new ServiceError('code_expired', 'This code has expired. Request a new one.');
		}
		if (challenge.attempts >= MAX_ATTEMPTS) {
			throw new ServiceError('rate_limited', 'Too many attempts. Request a new code.');
		}
		if (!isValidOtp(code) || challenge.code !== code) {
			challenge.attempts += 1;
			throw new ServiceError(
				'code_invalid',
				'That code is not right. Check the SMS and try again.'
			);
		}
		state.challenges = state.challenges.filter((c) => c !== challenge);
		return challenge;
	}

	function newUser(fields: Partial<User> & Pick<User, 'authProvider'>): User {
		const t = iso(now());
		return {
			id: newId('usr'),
			fullName: '',
			email: null,
			phone: null,
			phoneVerified: false,
			avatarUrl: null,
			role: 'community_scientist',
			platformAdmin: false,
			address: null,
			onboardingCompleted: false,
			createdAt: t,
			updatedAt: t,
			...fields
		};
	}

	return {
		auth: {
			async getCurrentUser() {
				const state = readDb();
				if (!state.session) return null;
				return state.users.find((u) => u.id === state.session!.userId) ?? null;
			},

			async signInWithGoogle(idToken) {
				await delay();
				// The mock ID token is the account's `sub`. The real backend verifies a signed JWT.
				return updateDb((state): AuthResult => {
					const account = state.googleAccounts.find((a) => a.sub === idToken);
					if (!account) throw new ServiceError('unauthenticated', 'Google sign-in failed.');
					const linked = state.users.find((u) => u.id === state.googleLinks[account.sub]);
					if (linked) {
						startSession(state, linked.id);
						return { user: linked, isNewUser: false };
					}
					const user = newUser({
						authProvider: 'google',
						fullName: account.name,
						email: account.email,
						avatarUrl: account.avatarUrl
					});
					state.users.push(user);
					state.googleLinks[account.sub] = user.id;
					startSession(state, user.id);
					return { user, isNewUser: true };
				});
			},

			async requestPhoneCode(phone) {
				await delay();
				return updateDb((state) => issueChallenge(state, phone, 'sign_in', null));
			},

			async verifyPhoneCode(requestId, code) {
				await delay();
				return updateDb((state): AuthResult => {
					const { phone } = consumeChallenge(state, requestId, code, 'sign_in');
					const existing = state.users.find((u) => u.phone === phone && u.phoneVerified);
					if (existing) {
						startSession(state, existing.id);
						return { user: existing, isNewUser: false };
					}
					const user = newUser({ authProvider: 'phone', phone, phoneVerified: true });
					state.users.push(user);
					startSession(state, user.id);
					return { user, isNewUser: true };
				});
			},

			async signOut() {
				await delay();
				updateDb((state) => {
					state.session = null;
				});
			}
		},

		profile: {
			async updateProfile(input) {
				await delay();
				const errors = validateProfile(input);
				if (Object.keys(errors).length) {
					throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
				}
				return updateDb((state) => {
					const user = currentUser(state);
					const email = input.email.trim().toLowerCase();
					if (state.users.some((u) => u.id !== user.id && u.email === email)) {
						throw new ServiceError('conflict', 'That email is already in use.', {
							email: 'That email is already in use.'
						});
					}
					user.fullName = input.fullName.trim();
					user.email = email;
					touch(user);
					return user;
				});
			},

			async startPhoneVerification(phone) {
				await delay();
				return updateDb((state) => {
					const user = currentUser(state);
					const normalized = normalizePhone(phone);
					if (normalized && state.users.some((u) => u.id !== user.id && u.phone === normalized)) {
						throw new ServiceError('conflict', 'That phone number belongs to another account.', {
							phone: 'That phone number belongs to another account.'
						});
					}
					return issueChallenge(state, phone, 'verify_phone', user.id);
				});
			},

			async confirmPhoneVerification(requestId, code) {
				await delay();
				return updateDb((state) => {
					const user = currentUser(state);
					const challenge = consumeChallenge(state, requestId, code, 'verify_phone');
					if (challenge.userId !== user.id) {
						throw new ServiceError('code_invalid', 'This code was issued for another account.');
					}
					user.phone = challenge.phone;
					user.phoneVerified = true;
					touch(user);
					return user;
				});
			},

			async updateAddress(address: Address) {
				await delay();
				const errors = validateAddress(address);
				if (Object.keys(errors).length) {
					throw new ServiceError('invalid_input', 'Check the highlighted fields.', errors);
				}
				return updateDb((state) => {
					const user = currentUser(state);
					user.address = { ...address };
					touch(user);
					return user;
				});
			},

			async completeOnboarding() {
				await delay();
				return updateDb((state) => {
					const user = currentUser(state);
					const missing = [
						!user.fullName && 'name',
						!user.email && 'email',
						!user.phoneVerified && 'verified phone',
						!user.address && 'address'
					].filter(Boolean);
					if (missing.length) {
						throw new ServiceError('invalid_input', `Still needed: ${missing.join(', ')}.`);
					}
					user.onboardingCompleted = true;
					touch(user);
					return user;
				});
			}
		},

		classrooms: createMockClassrooms({ delay, now }),

		labs: createMockLabs({ delay, now }),

		places: createMockPlaces(delay),

		registry: createMockRegistry({ delay, now })
	};
}
