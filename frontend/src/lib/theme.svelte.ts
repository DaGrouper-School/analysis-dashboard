export type Theme = 'light' | 'dark';

const KEY = 'dagrouper:theme';
const systemQuery =
	typeof matchMedia === 'undefined' ? null : matchMedia('(prefers-color-scheme: light)');

function stored(): Theme | null {
	try {
		const value = localStorage.getItem(KEY);
		return value === 'light' || value === 'dark' ? value : null;
	} catch {
		return null;
	}
}

function system(): Theme {
	return systemQuery?.matches ? 'light' : 'dark';
}

/** The active theme. app.html sets data-theme before the first paint; this mirrors it. */
export const theme = $state({ current: stored() ?? system() });

function apply(value: Theme) {
	theme.current = value;
	if (typeof document !== 'undefined') document.documentElement.dataset.theme = value;
}

/** An explicit choice is remembered; until then the theme follows the system setting. */
export function setTheme(value: Theme) {
	try {
		localStorage.setItem(KEY, value);
	} catch {
		// Private mode or blocked storage: the choice lasts for this visit only.
	}
	apply(value);
}

export function toggleTheme() {
	setTheme(theme.current === 'dark' ? 'light' : 'dark');
}

systemQuery?.addEventListener('change', () => {
	if (!stored()) apply(system());
});
