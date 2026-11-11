import prettier from 'eslint-config-prettier';
import path from 'node:path';
import js from '@eslint/js';
import svelte from 'eslint-plugin-svelte';
import { defineConfig, includeIgnoreFile } from 'eslint/config';
import globals from 'globals';
import ts from 'typescript-eslint';

const gitignorePath = path.resolve(import.meta.dirname, '.gitignore');

/**
 * Where code may import from (see "Where code goes" in the README). Product areas never import
 * each other: pages and `views` combine them. Building blocks (shared features, components,
 * utils) never import a product area or a view.
 */
const AREAS = ['auth', 'classroom', 'devices', 'alerts'];
const SHARED_FEATURES = ['measurements', 'map', 'consent', 'places'];
/** Refuses imports of the given `#lib` folders (a regex, since gitignore-style globs read `#` as a comment). */
const restrict = (files, folders, message) => ({
	files,
	rules: {
		'no-restricted-imports': [
			'error',
			{ patterns: [{ regex: `^#lib/(${folders.join('|')})/`, message }] }
		]
	}
});
const boundaries = [
	...AREAS.map((area) =>
		restrict(
			[`src/lib/features/${area}/**`],
			[...AREAS.filter((other) => other !== area).map((other) => `features/${other}`), 'views'],
			'Product areas do not import each other. Combine them in a page or in lib/views, or move the shared piece to a shared feature, components or utils.'
		)
	),
	restrict(
		SHARED_FEATURES.map((feature) => `src/lib/features/${feature}/**`),
		[...AREAS.map((area) => `features/${area}`), 'views'],
		'Shared features do not import product areas or views.'
	),
	restrict(
		['src/lib/components/**', 'src/lib/utils/**'],
		['features', 'views'],
		'Components and utils know nothing about the product: pass data in as props instead.'
	)
];

export default defineConfig(
	includeIgnoreFile(gitignorePath),
	js.configs.recommended,
	ts.configs.recommended,
	svelte.configs.recommended,
	prettier,
	svelte.configs.prettier,
	{
		languageOptions: { globals: { ...globals.browser, ...globals.node } },
		rules: {
			// typescript-eslint strongly recommend that you do not use the no-undef lint rule on TypeScript projects.
			// see: https://typescript-eslint.io/troubleshooting/faqs/eslint/#i-get-errors-from-the-no-undef-rule-about-global-variables-not-being-defined-even-though-there-are-no-typescript-errors
			'no-undef': 'off'
		}
	},
	{
		files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
		languageOptions: {
			parserOptions: {
				projectService: true,
				extraFileExtensions: ['.svelte'],
				parser: ts.parser
			}
		}
	},
	...boundaries,
	{
		// A file past 300 lines is flagged for a look: split it by what the reader sees.
		files: ['src/**/*.ts', 'src/**/*.svelte'],
		ignores: ['src/**/*.spec.ts', 'src/lib/data/**', 'src/lib/services/**', 'src/lib/domain/**'],
		rules: { 'max-lines': ['warn', { max: 300, skipBlankLines: true, skipComments: true }] }
	}
);
