import { resolve } from '$app/paths';

/**
 * A participant's public contributor page as a full address (DAG-143), for a CSV export, where a
 * path alone would not open.
 */
export function contributorUrl(participantId: number) {
	const path = resolve('/contributors/[participantId]', { participantId: String(participantId) });
	return new URL(path, location.origin).href;
}
