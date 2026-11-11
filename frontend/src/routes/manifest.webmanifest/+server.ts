import { asset, resolve } from '$app/paths';
import css from '../layout.css?raw';
import { canvasColors } from '#lib/features/app/theme-colors.ts';

// Written once at build time, so the static host serves it as a plain file.
export const prerender = true;

/**
 * The web app manifest that makes the dashboard installable. The paths follow the base path, so
 * the app installs the same on GitHub Pages and on its own domain. The icons in `static/icons`
 * are rendered from the logo, `lib/assets/favicon.svg`.
 */
export function GET() {
	const { dark } = canvasColors(css);
	const root = resolve('/');
	const manifest = {
		id: root,
		name: 'DaGrouper Analysis Dashboard',
		short_name: 'DaGrouper',
		description: 'Record and explore environmental readings with your classroom.',
		start_url: root,
		scope: root,
		display: 'standalone',
		orientation: 'any',
		background_color: dark,
		theme_color: dark,
		icons: [
			{ src: asset('icons/icon-192.png'), sizes: '192x192', type: 'image/png', purpose: 'any' },
			{ src: asset('icons/icon-512.png'), sizes: '512x512', type: 'image/png', purpose: 'any' },
			{
				src: asset('icons/icon-maskable-512.png'),
				sizes: '512x512',
				type: 'image/png',
				purpose: 'maskable'
			}
		]
	};
	return new Response(JSON.stringify(manifest, null, '\t'), {
		headers: { 'content-type': 'application/manifest+json' }
	});
}
