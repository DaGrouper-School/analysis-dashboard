/** The canvas colour of each theme, as declared in the stylesheet's tokens. */
export interface CanvasColors {
	dark: string;
	light: string;
}

const CANVAS = /--color-canvas:\s*([^;]+);/;

/**
 * Reads the canvas colours from the stylesheet text: the `@theme` block holds the dark default,
 * and the `[data-theme='light']` block swaps it. The installed app's splash screen and title bar
 * use them, so they always match the page.
 */
export function canvasColors(css: string): CanvasColors {
	const themeStart = css.indexOf('@theme');
	const lightStart = css.search(/\[data-theme=['"]?light['"]?\]/);
	const dark = themeStart >= 0 ? css.slice(themeStart).match(CANVAS)?.[1] : undefined;
	const light = lightStart >= 0 ? css.slice(lightStart).match(CANVAS)?.[1] : undefined;
	if (!dark || !light) throw new Error('The stylesheet has no --color-canvas for both themes.');
	return { dark: dark.trim(), light: light.trim() };
}
