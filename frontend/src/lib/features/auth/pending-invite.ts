/**
 * Remembers an invite link the user opened before signing in, so they land back on it
 * once sign-up and onboarding are done.
 */
const KEY = 'dagrouper:pending-invite';

export function savePendingInvite(token: string): void {
	try {
		sessionStorage.setItem(KEY, token);
	} catch {
		// Storage unavailable: the user can open the link again after signing up.
	}
}

export function takePendingInvite(): string | null {
	try {
		const token = sessionStorage.getItem(KEY);
		sessionStorage.removeItem(KEY);
		return token;
	} catch {
		return null;
	}
}
