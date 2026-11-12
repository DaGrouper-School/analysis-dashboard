import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => ({
	// Only the mock data source has demo accounts; real Google sign-in uses Google Identity Services.
	googleAccounts: await services.auth.demoGoogleAccounts(),
	dialCodes: await services.places.dialCodes()
});
