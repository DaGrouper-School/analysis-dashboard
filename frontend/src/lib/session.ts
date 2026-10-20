import { resolve } from '$app/paths';
import { goto, invalidate } from '$app/navigation';
import type { User } from '#lib/domain/types.ts';
import { nextRoute } from '#lib/routing.ts';

/** Reloads the session in the root layout, then moves the user to where they belong. */
export async function refreshSessionAndContinue(user: User | null): Promise<void> {
	await invalidate('app:session');
	await goto(resolve(nextRoute(user)), { replaceState: true });
}
