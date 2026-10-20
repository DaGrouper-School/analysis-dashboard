import type { User } from '#lib/domain/types.ts';

export function isProfileStepDone(user: User): boolean {
	return Boolean(user.fullName && user.email && user.phoneVerified);
}

/** Where a user belongs next in the sign-up flow (User Flow / 01 Signup screen). */
export type FlowRoute =
	'/(auth)/signup' | '/onboarding/profile' | '/onboarding/address' | '/(app)/dashboard';

export function nextRoute(user: User | null): FlowRoute {
	if (!user) return '/(auth)/signup';
	if (user.onboardingCompleted) return '/(app)/dashboard';
	if (!isProfileStepDone(user)) return '/onboarding/profile';
	return '/onboarding/address';
}
