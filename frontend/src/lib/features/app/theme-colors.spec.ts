import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { canvasColors } from './theme-colors.ts';

describe('canvasColors', () => {
	it('reads the dark default and the light swap', () => {
		const css = `@theme {\n\t--color-canvas: #000;\n}\n:root[data-theme='light'] {\n\t--color-canvas: #fafafa;\n}`;
		expect(canvasColors(css)).toEqual({ dark: '#000', light: '#fafafa' });
	});

	it('finds both colours in the app stylesheet', () => {
		const css = readFileSync(new URL('../../../routes/layout.css', import.meta.url), 'utf8');
		const { dark, light } = canvasColors(css);
		expect(dark).toMatch(/^#/);
		expect(light).toMatch(/^#/);
		expect(dark).not.toBe(light);
	});

	it('fails loudly when a token is missing', () => {
		expect(() => canvasColors('@theme {}')).toThrow();
	});
});
