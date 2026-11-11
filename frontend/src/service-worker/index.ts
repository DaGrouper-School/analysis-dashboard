import { assets, immutable, prerendered } from '$app/manifest';
import { self } from '$app/service-worker';

/**
 * Keeps the app working without a signal. The build's files and the static files are saved
 * when this worker installs, so the installed app opens offline; readings recorded meanwhile
 * wait in the outbox. Every build has its own cache, and the old one is removed once the new
 * worker takes over.
 */

// Build file names carry content hashes, so their list names the build.
const BUILD = immutable.map((file) => file.path).join('|');
let hash = 0;
for (let i = 0; i < BUILD.length; i++) hash = (hash * 31 + BUILD.charCodeAt(i)) | 0;
const CACHE = `dagrouper-${(hash >>> 0).toString(36)}`;
/** Fonts from Google Fonts, kept across builds. */
const FONTS = 'dagrouper-fonts';
const FONT_HOSTS = new Set(['fonts.googleapis.com', 'fonts.gstatic.com']);

/** The app's root, e.g. `https://example.org/analysis-dashboard/`. */
const scope = self.registration.scope;
const url = (path: string) => new URL(path, scope).href;
/** Every route renders in the browser, so the root page is the shell for any address. */
const SHELL = url('./');
/**
 * Script requests send an `Origin` header and hosts answer with `Vary: Origin`, which would stop
 * the saved copies (fetched without one) from matching. Saved files never vary, so ignore it.
 */
const MATCH: CacheQueryOptions = { ignoreVary: true };
const PRECACHE = [
	SHELL,
	...[...immutable, ...assets, ...prerendered]
		// Host files such as `.nojekyll` are not served.
		.filter((file) => !file.path.split('/').some((part) => part.startsWith('.')))
		.map((file) => url(file.path))
];

self.addEventListener('install', (event) => {
	event.waitUntil(
		caches
			.open(CACHE)
			.then((cache) => cache.addAll(PRECACHE))
			.then(() => self.skipWaiting())
	);
});

self.addEventListener('activate', (event) => {
	event.waitUntil(
		caches
			.keys()
			.then((keys) =>
				Promise.all(
					keys.filter((key) => key !== CACHE && key !== FONTS).map((key) => caches.delete(key))
				)
			)
			.then(() => self.clients.claim())
	);
});

self.addEventListener('fetch', (event) => {
	const request = event.request;
	if (request.method !== 'GET') return;
	const target = new URL(request.url);

	if (FONT_HOSTS.has(target.hostname)) {
		event.respondWith(staleWhileRevalidate(request));
		return;
	}
	if (target.origin !== self.location.origin || !request.url.startsWith(scope)) return;

	// Pages: the newest from the network, else the saved shell.
	if (request.mode === 'navigate') {
		event.respondWith(
			fetch(request).catch(async () => (await caches.match(SHELL, MATCH)) ?? Response.error())
		);
		return;
	}

	// Build and static files: saved at install, so serve the saved copy first.
	event.respondWith(caches.match(request, MATCH).then((saved) => saved ?? fetch(request)));
});

async function staleWhileRevalidate(request: Request): Promise<Response> {
	const cache = await caches.open(FONTS);
	const saved = await cache.match(request, MATCH);
	const fresh = fetch(request)
		.then((response) => {
			if (response.ok || response.type === 'opaque') cache.put(request, response.clone());
			return response;
		})
		.catch(() => saved ?? Response.error());
	return saved ?? fresh;
}
