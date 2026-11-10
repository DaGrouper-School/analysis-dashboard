import type { CoreReadings, Reading, StationCore } from '#lib/domain/types.ts';

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

function reviewOf(records: CoreRecord[], releasedThrough: string | null, ids: string[]) {
	const has = (r: CoreRecord) => firstNumber(r.values, ids) !== null;
	if (releasedThrough === null || !records.some((r) => has(r) && r.observedAt <= releasedThrough)) {
		return 'not_released' as const;
	}
	return records.some((r) => has(r) && r.observedAt > releasedThrough)
		? ('in_review' as const)
		: ('released' as const);
}

/** Each source's value on a day, by a rule: the running total's highest, or the latest. */
function bySourceOn(
	records: CoreRecord[],
	day: string | null,
	ids: string[],
	pick: 'max' | 'latest'
) {
	const values = new Map<string, { value: number; at: string }>();
	if (!day) return values;
	for (const record of records) {
		if (record.observedAt.slice(0, 10) !== day) continue;
		const value = firstNumber(record.values, ids);
		if (value === null) continue;
		const source = record.deviceId ?? 'field';
		const seen = values.get(source);
		const better = !seen || (pick === 'max' ? value > seen.value : record.observedAt > seen.at);
		if (better) values.set(source, { value, at: record.observedAt });
	}
	return values;
}

function spreadOf(values: number[]) {
	return values.length > 1 ? round(Math.max(...values) - Math.min(...values)) : null;
}

/**
 * The station page's detail on the core readings (AP-22): each reading's review state, how many
 * sources it comes from on the core day and how far apart they are, and today's temperature
 * range. Released data only, like `coreReadings`.
 */
export function stationCore(
	records: CoreRecord[],
	releasedThrough: string | null,
	parameters: CoreParameters
): StationCore {
	const released =
		releasedThrough === null ? [] : records.filter((r) => r.observedAt <= releasedThrough);
	const day =
		released
			.reduce<string | null>(
				(latest, r) => (!latest || r.observedAt > latest ? r.observedAt : latest),
				null
			)
			?.slice(0, 10) ?? null;

	const rain = [...bySourceOn(released, day, parameters.rainDaily, 'max').values()];
	const temperatures = [...bySourceOn(released, day, parameters.airTemperature, 'latest').values()];
	const today = released
		.filter((r) => r.observedAt.slice(0, 10) === day)
		.map((r) => firstNumber(r.values, parameters.airTemperature))
		.filter((t): t is number => t !== null);

	return {
		rain: {
			review: reviewOf(records, releasedThrough, parameters.rainDaily),
			sources: rain.length,
			spread: spreadOf(rain.map((r) => r.value))
		},
		temperature: {
			review: reviewOf(records, releasedThrough, parameters.airTemperature),
			sources: temperatures.length,
			spread: spreadOf(temperatures.map((t) => t.value)),
			low: today.length ? Math.min(...today) : null,
			high: today.length ? Math.max(...today) : null
		}
	};
}
