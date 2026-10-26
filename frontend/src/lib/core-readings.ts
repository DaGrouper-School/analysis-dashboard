import type { CoreReadings, Reading } from '#lib/domain/types.ts';

/** Which registry values feed the shared core readings. Kept with the data, not the page. */
export interface CoreParameters {
	/** Running rain totals for the day, e.g. a station's "Rain, today" or a gauge's daily total. */
	rainDaily: string[];
	airTemperature: string[];
}

type CoreRecord = Pick<Reading, 'observedAt' | 'receivedAt' | 'values'> & {
	/** The device, or null for a field observation. */
	deviceId: string | null;
};

const DAY = 86_400_000;
const round = (value: number) => Math.round(value * 10) / 10;

function firstNumber(values: Reading['values'], ids: string[]) {
	for (const id of ids) {
		const value = values[id];
		if (typeof value === 'number' && Number.isFinite(value)) return value;
	}
	return null;
}

/**
 * A day's rain for the classroom: each source's highest running total that day (the total only
 * grows until midnight), averaged over the sources that reported. Null when none did.
 */
function rainOn(records: CoreRecord[], day: string, ids: string[]) {
	const bySource = new Map<string, number>();
	for (const record of records) {
		if (record.observedAt.slice(0, 10) !== day) continue;
		const value = firstNumber(record.values, ids);
		if (value === null) continue;
		const source = record.deviceId ?? 'field';
		bySource.set(source, Math.max(bySource.get(source) ?? 0, value));
	}
	if (!bySource.size) return null;
	return round([...bySource.values()].reduce((a, b) => a + b, 0) / bySource.size);
}

/**
 * A classroom's core readings from released data only (AP-20): readings observed after
 * `releasedThrough` are still in review and count for nothing here. "Today" is the day of the
 * latest released reading, in UTC, so a quiet station shows its last day with its sync time
 * rather than a zero. Anything missing is null, never zero.
 */
export function coreReadings(
	records: CoreRecord[],
	releasedThrough: string | null,
	parameters: CoreParameters
): CoreReadings {
	const empty = {
		day: null,
		rainToday: null,
		rainLast7Days: null,
		rainDays: 0,
		airTemperature: null,
		lastSync: null
	};
	if (releasedThrough === null) return { ...empty, review: 'not_released' };
	const released = records
		.filter((r) => r.observedAt <= releasedThrough)
		.sort((a, b) => b.observedAt.localeCompare(a.observedAt));
	const review = records.some((r) => r.observedAt > releasedThrough) ? 'in_review' : 'released';
	const latest = released[0];
	if (!latest) return { ...empty, review };

	const day = latest.observedAt.slice(0, 10);
	const week = Array.from({ length: 7 }, (_, i) =>
		new Date(Date.parse(`${day}T00:00:00.000Z`) - i * DAY).toISOString().slice(0, 10)
	);
	const totals = week
		.map((d) => rainOn(released, d, parameters.rainDaily))
		.filter((t): t is number => t !== null);
	const temperature = released
		.map((r) => firstNumber(r.values, parameters.airTemperature))
		.find((t) => t !== null);

	return {
		day,
		rainToday: rainOn(released, day, parameters.rainDaily),
		rainLast7Days: totals.length ? round(totals.reduce((a, b) => a + b, 0)) : null,
		rainDays: totals.length,
		airTemperature: temperature ?? null,
		lastSync: latest.receivedAt,
		review
	};
}
