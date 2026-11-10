/**
 * Builds the backend API contract, `backend/openapi.yaml`, and its browsable page,
 * `backend/docs/index.html`, from three sources in the front end:
 *
 * - `src/lib/services/http/routes.ts`: method, path, access and body of every call;
 * - `src/lib/services/types.ts`: the service interfaces, whose JSDoc becomes the descriptions;
 * - `src/lib/domain/types.ts`: the data types, turned into JSON Schemas.
 *
 * Run `npm run openapi` after changing any of them. `npm run openapi:check` fails when the
 * committed files are out of date.
 */
import { createGenerator } from 'ts-json-schema-generator';
import ts from 'typescript';
import { stringify } from 'yaml';
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { routes, type Route } from '../src/lib/services/http/routes.ts';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const domainTypes = resolve(root, 'src/lib/domain/types.ts');
const serviceTypes = resolve(root, 'src/lib/services/types.ts');
const specFile = resolve(root, '../backend/openapi.yaml');
const pageFile = resolve(root, '../backend/docs/index.html');

const SWAGGER_UI = 'https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14';

const TAGS: Record<string, string> = {
	auth: 'Auth',
	profile: 'Profile',
	classrooms: 'Classrooms',
	labs: 'Labs and devices',
	exports: 'Exports',
	publicSite: 'Public site',
	consent: 'Guardian consent',
	hosting: 'Hosting',
	places: 'Places',
	registry: 'Registry'
};

/** For services whose interface JSDoc does not describe the group. */
const TAG_NOTES: Record<string, string> = {
	auth: 'Sign-in with Google or a phone code, and the session cookie.',
	profile: "The signed-in user's own profile, phone, address, onboarding and guardian consent.",
	places: 'Address lookup, proxied so the provider key stays on the server.'
};

const ACCESS_NOTE: Record<Route['access'], string> = {
	public: 'Access: public, no session needed.',
	user: 'Access: signed-in users. Classroom roles are checked on the server.',
	admin: 'Access: platform admins only.'
};

type Schema = Record<string, unknown>;

/** JSDoc of each service interface and its methods, keyed by the `Services` property name. */
function readServiceDocs() {
	const source = ts.createSourceFile(
		serviceTypes,
		readFileSync(serviceTypes, 'utf8'),
		ts.ScriptTarget.Latest,
		true
	);
	const interfaces = new Map<string, ts.InterfaceDeclaration>();
	source.forEachChild((node) => {
		if (ts.isInterfaceDeclaration(node)) interfaces.set(node.name.text, node);
	});
	const doc = (node: ts.Node) => {
		const tags = (node as { jsDoc?: ts.JSDoc[] }).jsDoc;
		return tags?.length ? (ts.getTextOfJSDocComment(tags.at(-1)!.comment) ?? '') : '';
	};
	const services = new Map<string, { doc: string; methods: Map<string, string> }>();
	for (const member of interfaces.get('Services')!.members) {
		if (!ts.isPropertySignature(member) || !member.type) continue;
		const name = (member.name as ts.Identifier).text;
		const iface = interfaces.get(member.type.getText(source))!;
		const methods = new Map<string, string>();
		for (const m of iface.members) {
			if (ts.isMethodSignature(m)) methods.set((m.name as ts.Identifier).text, doc(m));
		}
		services.set(name, { doc: doc(iface), methods });
	}
	const codes = source.statements.find(
		(s): s is ts.TypeAliasDeclaration =>
			ts.isTypeAliasDeclaration(s) && s.name.text === 'ServiceErrorCode'
	)!;
	const errorCodes = codes.type
		.getText(source)
		.split('|')
		.map((c) => c.trim().replace(/'/g, ''))
		.filter((c) => c && c !== 'network');
	return { services, errorCodes };
}

const pascal = (s: string) =>
	s.replace(/(^|[.\s_-])(\w)/g, (_, __, c: string) => c.toUpperCase()).replace(/\W/g, '');

const isName = (expr: string) => /^[A-Z]\w*$/.test(expr);

/** Turns the route type expressions into JSON Schemas, all under `components.schemas`. */
function buildSchemas() {
	const aliases: string[] = [];
	const names = new Set<string>();
	const refs = new Map<string, Schema>();

	const ref = (name: string): Schema => {
		names.add(name);
		return { $ref: `#/components/schemas/${name}` };
	};
	const typeSchema = (expr: string, alias: string): Schema => {
		const array = expr.match(/^([A-Z]\w*)\[\]$/);
		if (array) return { type: 'array', items: ref(array[1]) };
		const nullable = expr.match(/^([A-Z]\w*) \| null$/);
		if (nullable) return { oneOf: [ref(nullable[1]), { type: 'null' }] };
		if (isName(expr)) return ref(expr);
		aliases.push(`export type ${alias} = ${expr};`);
		return ref(alias);
	};

	for (const route of routes) {
		const id = pascal(route.operationId ?? route.method);
		if (route.body) refs.set(`${id}:body`, typeSchema(route.body, `${id}Body`));
		if (route.response) refs.set(`${id}:response`, typeSchema(route.response, `${id}Response`));
		for (const [name, param] of Object.entries(route.query ?? {})) {
			if (param.ts)
				refs.set(`${id}:query:${name}`, typeSchema(param.ts, `${id}Query${pascal(name)}`));
		}
	}

	const tmp = resolve(root, 'scripts/.openapi-types.ts');
	// A copy of the domain types plus the aliases, so the generator sees one self-contained file.
	writeFileSync(tmp, [readFileSync(domainTypes, 'utf8'), ...aliases, ''].join('\n'));
	const definitions: Record<string, Schema> = {};
	try {
		const generator = createGenerator({
			path: tmp,
			expose: 'export',
			topRef: true,
			jsDoc: 'extended',
			skipTypeCheck: true,
			additionalProperties: false
		});
		for (const name of [...names].sort()) {
			Object.assign(definitions, generator.createSchema(name).definitions);
		}
	} finally {
		rmSync(tmp);
	}
	const schemas = JSON.parse(
		JSON.stringify(definitions).replaceAll('#/definitions/', '#/components/schemas/')
	) as Record<string, Schema>;
	const sorted = Object.fromEntries(Object.entries(schemas).sort(([a], [b]) => a.localeCompare(b)));
	return { schemas: sorted, refs };
}

function buildSpec() {
	const { services, errorCodes } = readServiceDocs();
	const { schemas, refs } = buildSchemas();
	const pkg = JSON.parse(readFileSync(resolve(root, 'package.json'), 'utf8'));

	const error = (description: string) => ({
		description,
		content: { 'application/json': { schema: { $ref: '#/components/schemas/Error' } } }
	});
	const ERRORS = {
		400: [
			'BadRequest',
			'The input is invalid (`invalid_input`), or a code is wrong or old (`code_invalid`, `code_expired`). `fields` holds a message per input.'
		],
		401: ['Unauthenticated', 'No valid session (`unauthenticated`).'],
		403: ['Forbidden', 'Signed in, but not allowed (`forbidden`).'],
		404: ['NotFound', 'No such resource, or not visible to this user (`not_found`).'],
		409: ['Conflict', 'Clashes with existing data or a decision already made (`conflict`).'],
		429: ['RateLimited', 'Too many requests (`rate_limited`). Try again later.']
	} as const;

	const paths: Record<string, Record<string, unknown>> = {};
	for (const route of routes) {
		const [service, method] = route.method.split('.');
		const id = pascal(route.operationId ?? route.method);
		const description = [
			services.get(service)?.methods.get(method),
			route.notes,
			ACCESS_NOTE[route.access]
		]
			.filter(Boolean)
			.join('\n\n');
		const pathParams = [...route.path.matchAll(/\{(\w+)\}/g)].map((m) => m[1]);
		const parameters = [
			...pathParams.map((name) => ({
				name,
				in: 'path',
				required: true,
				schema: { type: 'string' }
			})),
			...Object.entries(route.query ?? {}).map(([name, p]) => ({
				name,
				in: 'query',
				required: p.required ?? false,
				...(p.description ? { description: p.description } : {}),
				schema: p.ts ? refs.get(`${id}:query:${name}`) : { type: p.type }
			}))
		];

		const statuses = new Set<number>(route.errors ?? []);
		if (route.access !== 'public') statuses.add(401).add(403);
		if (pathParams.length) statuses.add(404);
		const responses: Record<string, unknown> = route.response
			? {
					200: {
						description: 'OK',
						content: { 'application/json': { schema: refs.get(`${id}:response`) } }
					}
				}
			: { 204: { description: 'No content' } };
		for (const status of [...statuses].sort()) {
			responses[status] = {
				$ref: `#/components/responses/${ERRORS[status as keyof typeof ERRORS][0]}`
			};
		}

		paths[route.path] ??= {};
		paths[route.path][route.http.toLowerCase()] = {
			tags: [TAGS[service]],
			operationId: route.operationId ?? route.method,
			summary: summary(route.operationId ?? route.method),
			description,
			...(route.access === 'public' ? { security: [] } : {}),
			...(parameters.length ? { parameters } : {}),
			...(route.body
				? {
						requestBody: {
							required: true,
							content: { 'application/json': { schema: refs.get(`${id}:body`) } }
						}
					}
				: {}),
			responses
		};
	}

	return {
		openapi: '3.1.0',
		info: {
			title: 'DaGrouper School backend API',
			version: pkg.version,
			description: [
				'The API the front end calls. The front end never calls SAM; this backend does, and keeps SAM behind it.',
				'**Status:** proposal to agree on (BE-01). The front end runs on a mock with the same interfaces; set `VITE_DATA_SOURCE=http` and `VITE_API_URL` to call this API instead.',
				'**Sessions:** an HTTP-only cookie set by the sign-in routes. The front end sends `credentials: include` and never handles tokens.',
				'**Errors:** every failure returns `Error`: a `code`, a `message` the UI can show, and `fields` with a message per input.',
				'**Generated** from `frontend/src/lib/services` by `npm run openapi`. Do not edit by hand.'
			].join('\n\n')
		},
		servers: [{ url: '/api', description: 'Same origin as the front end' }],
		security: [{ session: [] }],
		tags: Object.entries(TAGS).map(([key, name]) => ({
			name,
			description: TAG_NOTES[key] ?? services.get(key)?.doc
		})),
		paths,
		components: {
			securitySchemes: {
				session: {
					type: 'apiKey',
					in: 'cookie',
					name: 'session',
					description: 'HTTP-only session cookie.'
				}
			},
			responses: Object.fromEntries(
				Object.values(ERRORS).map(([name, text]) => [name, error(text)])
			),
			schemas: {
				Error: {
					type: 'object',
					properties: {
						code: { type: 'string', enum: errorCodes },
						message: { type: 'string', description: 'Shown to the user as is.' },
						fields: {
							type: 'object',
							additionalProperties: { type: 'string' },
							description: 'Messages keyed by input name.'
						}
					},
					required: ['code', 'message'],
					additionalProperties: false
				},
				...schemas
			}
		}
	};
}

/** `labs.createAlert` → "Create alert". */
function summary(operationId: string) {
	const words = operationId
		.split('.')[1]
		.replace(/([A-Z])/g, ' $1')
		.toLowerCase();
	return words[0].toUpperCase() + words.slice(1);
}

function page(spec: unknown) {
	const json = JSON.stringify(spec).replaceAll('</', '<\\/');
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>DaGrouper School API</title>
<link rel="stylesheet" href="${SWAGGER_UI}/swagger-ui.css">
<style>body { margin: 0; background: #fff; }</style>
</head>
<body>
<!-- Generated by \`npm run openapi\` in frontend/. Do not edit by hand. -->
<div id="swagger-ui"></div>
<script src="${SWAGGER_UI}/swagger-ui-bundle.js"></script>
<script>
SwaggerUIBundle({ spec: ${json}, dom_id: '#swagger-ui', deepLinking: true, docExpansion: 'none', defaultModelsExpandDepth: 0, supportedSubmitMethods: [] });
</script>
</body>
</html>
`;
}

const spec = buildSpec();
const yaml = `# Generated by \`npm run openapi\` in frontend/. Do not edit by hand.\n${stringify(spec, { lineWidth: 100 })}`;
const html = page(spec);

if (process.argv.includes('--check')) {
	const stale = [
		[specFile, yaml],
		[pageFile, html]
	].filter(([file, text]) => !existsSync(file) || readFileSync(file, 'utf8') !== text);
	if (stale.length) {
		console.error(`Out of date: ${stale.map(([f]) => f).join(', ')}. Run \`npm run openapi\`.`);
		process.exit(1);
	}
	console.log('API contract is up to date.');
} else {
	mkdirSync(dirname(pageFile), { recursive: true });
	writeFileSync(specFile, yaml);
	writeFileSync(pageFile, html);
	console.log(`Wrote ${specFile} and ${pageFile} (${routes.length} routes).`);
}
