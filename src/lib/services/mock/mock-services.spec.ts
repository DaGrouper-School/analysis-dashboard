import { beforeEach, describe, expect, it } from 'vitest';
import { resetDb } from '#lib/data/mock-db.ts';
import { ServiceError } from '../types';
import { createMockServices } from './index';

let clock = Date.parse('2026-10-20T10:00:00Z');
const services = createMockServices({ latency: 0, now: () => clock });

beforeEach(() => {
	resetDb();
	clock = Date.parse('2026-10-20T10:00:00Z');
});

async function signUpWithPhone(phone: string) {
	const challenge = await services.auth.requestPhoneCode(phone);
	return services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!);
}

describe('phone sign-up (AP-01)', () => {
	it('creates a new user with a verified phone', async () => {
		const { user, isNewUser } = await signUpWithPhone('+251 922 123 456');
		expect(isNewUser).toBe(true);
		expect(user.phone).toBe('+251922123456');
		expect(user.phoneVerified).toBe(true);
		expect(await services.auth.getCurrentUser()).toEqual(user);
	});

	it('signs an existing user back in', async () => {
		const { user, isNewUser } = await signUpWithPhone('+251911000001');
		expect(isNewUser).toBe(false);
		expect(user.id).toBe('usr_abebe');
		expect(user.onboardingCompleted).toBe(true);
	});

	it('rejects invalid numbers', async () => {
		await expect(services.auth.requestPhoneCode('12')).rejects.toMatchObject({
			code: 'invalid_input'
		});
	});

	it('enforces the 30 second resend timer', async () => {
		await services.auth.requestPhoneCode('+251922123456');
		await expect(services.auth.requestPhoneCode('+251922123456')).rejects.toMatchObject({
			code: 'rate_limited'
		});
		clock += 30_000;
		await expect(services.auth.requestPhoneCode('+251922123456')).resolves.toBeTruthy();
	});

	it('rejects a wrong code and locks after five attempts', async () => {
		const challenge = await services.auth.requestPhoneCode('+251922123456');
		const wrong = challenge.devCode === '000000' ? '111111' : '000000';
		for (let i = 0; i < 5; i++) {
			await expect(services.auth.verifyPhoneCode(challenge.requestId, wrong)).rejects.toMatchObject(
				{ code: 'code_invalid' }
			);
		}
		await expect(
			services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!)
		).rejects.toMatchObject({ code: 'rate_limited' });
	});

	it('expires codes after five minutes', async () => {
		const challenge = await services.auth.requestPhoneCode('+251922123456');
		clock += 5 * 60_000;
		await expect(
			services.auth.verifyPhoneCode(challenge.requestId, challenge.devCode!)
		).rejects.toMatchObject({ code: 'code_expired' });
	});
});

describe('Google sign-up (AP-01)', () => {
	it('pre-fills name and email for a new account', async () => {
		const { user, isNewUser } = await services.auth.signInWithGoogle('g-hana');
		expect(isNewUser).toBe(true);
		expect(user).toMatchObject({ fullName: 'Hana Bekele', email: 'hana.bekele@example.org' });
		expect(user.phoneVerified).toBe(false);
	});

	it('signs an existing account straight in', async () => {
		const { user, isNewUser } = await services.auth.signInWithGoogle('g-sarah');
		expect(isNewUser).toBe(false);
		expect(user.onboardingCompleted).toBe(true);
	});
});

describe('onboarding (AP-02)', () => {
	it('walks a Google user through phone, profile and address', async () => {
		await services.auth.signInWithGoogle('g-amina');
		await expect(services.profile.completeOnboarding()).rejects.toBeInstanceOf(ServiceError);

		const challenge = await services.profile.startPhoneVerification('+252612345678');
		await services.profile.confirmPhoneVerification(challenge.requestId, challenge.devCode!);
		await services.profile.updateProfile({ fullName: 'Amina Yusuf', email: 'amina@example.org' });
		const [place] = await services.places.autocomplete('Hargeisa');
		await services.profile.updateAddress(await services.places.getAddress(place.placeId));

		const user = await services.profile.completeOnboarding();
		expect(user).toMatchObject({
			onboardingCompleted: true,
			phone: '+252612345678',
			address: { city: 'Hargeisa', country: 'Somalia' }
		});
	});

	it('refuses an email that another account uses', async () => {
		await signUpWithPhone('+251922123456');
		await expect(
			services.profile.updateProfile({ fullName: 'New Person', email: 'sarah.k@example.org' })
		).rejects.toMatchObject({ code: 'conflict', fields: { email: expect.any(String) } });
	});

	it('requires a session', async () => {
		await expect(
			services.profile.updateProfile({ fullName: 'Nobody', email: 'nobody@example.org' })
		).rejects.toMatchObject({ code: 'unauthenticated' });
	});
});

describe('registry (SAM-01, SAM-05)', () => {
	it('registers the 36 station parameters plus the manual rain gauge across six domains', async () => {
		const parameters = await services.registry.listParameters();
		expect(parameters).toHaveLength(37);
		expect(new Set(parameters.map((p) => p.domain)).size).toBe(6);
		expect(new Set(parameters.map((p) => p.id)).size).toBe(parameters.length);
	});
});
