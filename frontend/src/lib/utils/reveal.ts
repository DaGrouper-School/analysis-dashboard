/**
 * Brings a sensor's values into view after it is picked, unless they are already on screen.
 * The sticky app header is allowed for through the element's `scroll-margin-top`. Reduced
 * motion jumps instead of gliding.
 */
export function reveal(element: HTMLElement | null | undefined) {
	if (!element) return;
	const header = document.querySelector('header')?.getBoundingClientRect().bottom ?? 0;
	const { top, bottom } = element.getBoundingClientRect();
	// Already in view: wholly on screen, or starting in the upper part of it under the header.
	const visible = top >= header && (bottom <= innerHeight || top <= innerHeight * 0.4);
	if (visible) return;
	const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
	element.scrollIntoView({ behavior: reduce ? 'auto' : 'smooth', block: 'start' });
}
