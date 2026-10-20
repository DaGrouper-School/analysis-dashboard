import { createHttpServices } from './http';
import { createMockServices } from './mock';
import type { Services } from './types';

export * from './types';

/**
 * The one place that decides where data comes from.
 * `VITE_DATA_SOURCE=http` with `VITE_API_URL` switches to the real backend; mock is the default.
 */
function createServices(): Services {
	if (import.meta.env.VITE_DATA_SOURCE === 'http') {
		return createHttpServices(import.meta.env.VITE_API_URL ?? '/api');
	}
	return createMockServices();
}

export const services: Services = createServices();

export const isMockDataSource = import.meta.env.VITE_DATA_SOURCE !== 'http';
