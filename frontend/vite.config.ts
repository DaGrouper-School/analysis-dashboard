import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import pkg from './package.json' with { type: 'json' };

export default defineConfig({
	// The app version shown in the site footer, from package.json.
	define: { __APP_VERSION__: JSON.stringify(pkg.version) },
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			// SPA mode: no server yet, every route falls back to the client-side app.
			// GitHub Pages serves 404.html for unknown paths, which boots the app on any route.
			adapter: adapter({ fallback: '404.html' }),
			// Set by the Pages workflow to the repo path, e.g. /analysis-dashboard.
			// Absolute asset paths: the offline app shell is one saved page served at every address,
			// so its script links must not depend on the address it is shown at.
			paths: { base: (process.env.BASE_PATH ?? '') as '' | `/${string}`, relative: false },
			// Look for a newly published version every 10 minutes, so open and installed apps offer to
			// reload (as well as when the tab comes back into view).
			version: { pollInterval: 10 * 60_000 }
		})
	],
	test: {
		expect: { requireAssertions: true },
		projects: [
			{
				extends: './vite.config.ts',
				test: {
					name: 'server',
					environment: 'node',
					include: ['src/**/*.{test,spec}.{js,ts}'],
					exclude: ['src/**/*.svelte.{test,spec}.{js,ts}']
				}
			}
		]
	}
});
