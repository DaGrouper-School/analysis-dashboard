import { readDb } from '#lib/data/mock-db.ts';
import { isMockDataSource } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = () => ({
	// Only the mock data source has a picker; real Google sign-in uses Google Identity Services.
	googleAccounts: isMockDataSource ? readDb().googleAccounts : []
});
