import { readDb } from '#lib/data/mock-db.ts';
import { isMockDataSource, services } from '#lib/services/index.ts';
import { USER_ROLE_LABELS } from '#lib/utils/labels.ts';
import type { PageLoad } from './$types';

/** A demo Google account's type: the linked user's role, or new when no user has it yet. */
function accountsWithType() {
	const state = readDb();
	return state.googleAccounts.map((account) => {
		const user = state.users.find((u) => u.id === state.googleLinks[account.sub]);
		const type = !user
			? 'New account'
			: user.platformAdmin
				? 'Platform admin'
				: USER_ROLE_LABELS[user.role];
		return { ...account, type };
	});
}

export const load: PageLoad = async () => ({
	// Only the mock data source has a picker; real Google sign-in uses Google Identity Services.
	googleAccounts: isMockDataSource ? accountsWithType() : [],
	dialCodes: await services.places.dialCodes()
});
