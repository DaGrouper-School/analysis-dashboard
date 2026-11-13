import { vi } from 'vitest';

/**
 * Shared set-up for component tests in the browser: the mock services with no simulated latency,
 * and SvelteKit's navigation, which only exists inside the running app, as spies.
 */
vi.mock('#lib/services/index.ts', async (importOriginal) => {
	const original = await importOriginal<typeof import('#lib/services/index.ts')>();
	const { createMockServices } = await import('#lib/services/mock/index.ts');
	return { ...original, services: createMockServices({ latency: 0 }) };
});

vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	invalidate: vi.fn(),
	invalidateAll: vi.fn()
}));

vi.mock('$app/paths', () => ({
	/** `/classrooms/[id]/setup` with `{ id: 'x' }` gives `/classrooms/x/setup`; groups drop out. */
	resolve: (route: string, params: Record<string, string> = {}) =>
		route
			.replace(/\/\([^)]+\)/g, '')
			.replace(/\[(\w+)\]/g, (_, key: string) => params[key] ?? '') || '/'
}));
