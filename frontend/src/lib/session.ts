import { goto, invalidate } from '$app/navigation';
import { resolve } from '$app/paths';
import type { User } from '#lib/domain/types.ts';
import { takePendingInvite } from '#lib/pending-invite.ts';
import { nextRoute } from '#lib/routing.ts';

/** Reloads the session in the root layout, then moves the user to where they belong. */
export async function refreshSessionAndContinue(user: User | null): Promise<void> {
	await invalidate('app:session');
	// Back to the invite link that started the sign-up, once the account is ready.
	const invite = user?.onboardingCompleted ? takePendingInvite() : null;
	if (invite) {
		await goto(resolve('/invite/[token]', { token: invite }), { replaceState: true });
		return;
	}
	await goto(resolve(nextRoute(user)), { replaceState: true });
}
