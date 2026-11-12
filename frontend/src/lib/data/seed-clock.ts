import type { MockDbState } from './mock-db';

const DAY = 86_400_000;

/**
 * Moves the seeded readings and observations forward by whole days, so the latest one falls on
 * today and the charts' history always runs up to now. Whole days keep each reading at the same
 * hour, so UV and sunlight still follow the sun. Only seeded records move: what people synced or
 * recorded keeps its own time. A release date that covered the seed moves with it. Returns
 * whether anything moved.
 */
export function rollSeedForward(state: MockDbState, now: number): boolean {
	const latest = Date.parse(state.seedClock.latest);
	const days = Math.floor((now - latest) / DAY);
	if (days <= 0) return false;
	const shift = (iso: string) => new Date(Date.parse(iso) + days * DAY).toISOString();
	const readingIds = new Set(state.seedClock.readingIds);
	const observationIds = new Set(state.seedClock.observationIds);
	for (const record of [
		...state.readings.filter((r) => readingIds.has(r.id)),
		...state.observations.filter((o) => observationIds.has(o.id))
	]) {
		record.observedAt = shift(record.observedAt);
		record.receivedAt = shift(record.receivedAt);
	}
	for (const classroom of state.classrooms) {
		if (classroom.releasedThrough && Date.parse(classroom.releasedThrough) <= latest) {
			classroom.releasedThrough = shift(classroom.releasedThrough);
		}
	}
	state.seedClock.latest = shift(state.seedClock.latest);
	return true;
}
