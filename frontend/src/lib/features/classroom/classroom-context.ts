/**
 * The classroom picked in the top bar: the context the dashboard, domains and labs pages show.
 * Kept in this browser so it holds across pages and visits; `?classroom=` in an address wins.
 */
const KEY = 'dagrouper:classroom';

export function rememberClassroom(id: string) {
	try {
		localStorage.setItem(KEY, id);
	} catch {
		// Private windows can refuse storage; the address still carries the pick.
	}
}

function remembered() {
	try {
		return localStorage.getItem(KEY);
	} catch {
		return null;
	}
}

/** The classroom in context: the address's, else the one picked last, else `fallback`, else the first. */
export function contextClassroom<T extends { id: string }>(
	classrooms: T[],
	url: URL,
	fallback?: string | null
): T | null {
	return (
		classrooms.find((c) => c.id === url.searchParams.get('classroom')) ??
		classrooms.find((c) => c.id === remembered()) ??
		classrooms.find((c) => c.id === fallback) ??
		classrooms[0] ??
		null
	);
}
