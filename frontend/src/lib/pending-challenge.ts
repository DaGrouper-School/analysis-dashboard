import type { PhoneChallenge } from '#lib/domain/types.ts';

/**
 * Carries the SMS challenge from /signup to /verify-phone. Kept in sessionStorage so a
 * reload of the verify page still works; no secrets live here with a real backend.
 */
const KEY = 'dagrouper:pending-challenge';

export function savePendingChallenge(challenge: PhoneChallenge): void {
	try {
		sessionStorage.setItem(KEY, JSON.stringify(challenge));
	} catch {
		// Storage unavailable: the verify page will send the user back to sign-up.
	}
}

export function loadPendingChallenge(): PhoneChallenge | null {
	try {
		const raw = sessionStorage.getItem(KEY);
		return raw ? (JSON.parse(raw) as PhoneChallenge) : null;
	} catch {
		return null;
	}
}

export function clearPendingChallenge(): void {
	try {
		sessionStorage.removeItem(KEY);
	} catch {
		// Nothing to clear.
	}
}
