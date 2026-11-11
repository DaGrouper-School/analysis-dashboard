import { services } from '#lib/services/index.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => ({ dialCodes: await services.places.dialCodes() });
