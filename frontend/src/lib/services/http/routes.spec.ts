import { afterEach, describe, expect, it, vi } from 'vitest';
import { createHttpServices } from './index';
import { routes } from './routes';

interface Call {
	method: string;
	path: string;
	query: string[];
	hasBody: boolean;
}

function recordCall(route: (typeof routes)[number]): Promise<Call> {
	let call: Call | undefined;
	vi.stubGlobal('fetch', async (url: string, init: RequestInit) => {
		const parsed = new URL(url, 'http://api.test');
		call = {
			method: init.method ?? 'GET',
			path: parsed.pathname.replace(/^\/api/, ''),
			query: [...parsed.searchParams.keys()].sort(),
			hasBody: init.body !== undefined
		};
		return new Response(null, { status: 204 });
	});
	const [service, method] = route.method.split('.');
	const services = createHttpServices('/api') as unknown as Record<
		string,
		Record<string, (...args: unknown[]) => Promise<unknown>>
	>;
	const args = route.args ?? ['x1', 'x2', 'x3', 'x4'];
	return services[service][method](...args).then(() => {
		if (!call) throw new Error(`${route.method} made no request`);
		return call;
	});
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('HTTP routes', () => {
	it('lists every service method', () => {
		const services = createHttpServices('/api') as unknown as Record<string, object>;
		const methods = Object.entries(services).flatMap(([name, service]) =>
			Object.keys(service).map((method) => `${name}.${method}`)
		);
		expect(new Set(routes.map((r) => r.method))).toEqual(new Set(methods));
	});

	it('has unique operations', () => {
		const keys = routes.map((r) => `${r.http} ${r.path}`);
		expect(new Set(keys).size).toBe(keys.length);
		const ids = routes.map((r) => r.operationId ?? r.method);
		expect(new Set(ids).size).toBe(ids.length);
	});

	it.each(routes.map((r) => [r.operationId ?? r.method, r] as const))(
		'%s matches what the client sends',
		async (_, route) => {
			const call = await recordCall(route);
			const template = route.path.replace(/\{[^}]+\}/g, '{}');
			expect(`${call.method} ${call.path.replace(/x\d/g, '{}')}`).toBe(`${route.http} ${template}`);
			expect(call.query).toEqual(Object.keys(route.query ?? {}).sort());
			expect(call.hasBody).toBe(route.body !== undefined);
		}
	);
});
