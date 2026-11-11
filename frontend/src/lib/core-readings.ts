import type {
	CoreReadings,
	Reading,
	StationCharts,
	StationCore,
	StationRainDay
} from '#lib/domain/types.ts';

/** Which registry values feed the shared core readings. Kept with the data, not the page. */
export interface CoreParameters {
	/** Running rain totals for the day, e.g. a station's "Rain, today" or a gauge's daily total. */
	rainDaily: string[];
	airTemperature: string[];
	/**
	 * How far apart two rain sources may be on a day, in mm, before the day goes to review
	 * (AP-23). From the classroom protocol; the manual gauge is the reference.
	 */
	rainTolerance: number;
	/** The manual gauge's value: the reference for a day's rain wherever a gauge reported. */
	rainReference: string;
	/** The values the classroom charts follow besides rain and air temperature. */
	uv: string[];
	soilMoisture: string[];
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
 * A day's rain with where it came from (AP-23): the reference gauge's total when the gauge
 * reported, otherwise the first other source's, and whether the sources are further apart than
 * the protocol allows. Null when no source reported.
 */
function rainDayOn(records: CoreRecord[], day: string, parameters: CoreParameters): StationRainDay {
	const bySource = new Map<string, { value: number; reference: boolean }>();
	for (const record of records) {
		if (record.observedAt.slice(0, 10) !== day) continue;
		const value = firstNumber(record.values, parameters.rainDaily);
		if (value === null) continue;
		const source = record.deviceId ?? 'field';
		const seen = bySource.get(source);
		bySource.set(source, {
			value: Math.max(seen?.value ?? 0, value),
			reference: !!seen?.reference || typeof record.values[parameters.rainReference] === 'number'
		});
	}
	const values = [...bySource.values()];
	const shown = values.find((v) => v.reference) ?? values[0];
	const spread = spreadOf(values.map((v) => v.value));
	return {
		day,
		value: shown ? round(shown.value) : null,
		reference: !!shown?.reference,
		flagged: spread !== null && spread > parameters.rainTolerance
	};
}

/** The `count` days up to and including `day`, oldest first, as `YYYY-MM-DD`. */
function daysUpTo(day: string, count: number) {
	const end = Date.parse(`${day}T00:00:00.000Z`);
	return Array.from({ length: count }, (_, i) =>
		new Date(end - (count - 1 - i) * DAY).toISOString().slice(0, 10)
	);
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
		.map((d) => rainDayOn(released, d, parameters).value)
		.filter((t): t is number => t !== null);
	const temperature = released
		.map((r) => firstNumber(r.values, parameters.airTemperature))
		.find((t) => t !== null);

	return {
		day,
		rainToday: rainDayOn(released, day, parameters).value,
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
	parameters: CoreParameters,
	nameOf: (sourceId: string) => string | null = () => null
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

	const rainBySource = bySourceOn(released, day, parameters.rainDaily, 'max');
	const rain = [...rainBySource.values()];
	const isReference = (sourceId: string) =>
		released.some(
			(r) =>
				(r.deviceId ?? 'field') === sourceId &&
				typeof r.values[parameters.rainReference] === 'number'
		);
	const month = day
		? released
				.filter((r) => r.observedAt.slice(0, 10) > daysUpTo(day, 31)[0])
				.map((r) => firstNumber(r.values, parameters.airTemperature))
				.filter((t): t is number => t !== null)
		: [];
	const temperatures = [...bySourceOn(released, day, parameters.airTemperature, 'latest').values()];
	const today = released
		.filter((r) => r.observedAt.slice(0, 10) === day)
		.map((r) => firstNumber(r.values, parameters.airTemperature))
		.filter((t): t is number => t !== null);

	return {
		rain: {
			review: reviewOf(records, releasedThrough, parameters.rainDaily),
			sources: rain.length,
			spread: spreadOf(rain.map((r) => r.value)),
			bySource: [...rainBySource.entries()]
				.map(([sourceId, r]) => ({
					sourceId,
					name: nameOf(sourceId),
					value: round(r.value),
					reference: isReference(sourceId)
				}))
				.sort((a, b) => Number(b.reference) - Number(a.reference)),
			tolerance: parameters.rainTolerance,
			week: day ? daysUpTo(day, 7).map((d) => rainDayOn(released, d, parameters)) : []
		},
		temperature: {
			review: reviewOf(records, releasedThrough, parameters.airTemperature),
			sources: temperatures.length,
			spread: spreadOf(temperatures.map((t) => t.value)),
			low: today.length ? Math.min(...today) : null,
			high: today.length ? Math.max(...today) : null,
			monthLow: month.length ? Math.min(...month) : null,
			monthHigh: month.length ? Math.max(...month) : null
		}
	};
}

/** The day's numbers of one value, from every source, in time order. */
function dayValues(records: CoreRecord[], day: string, ids: string[]) {
	return records
		.filter((r) => r.observedAt.slice(0, 10) === day)
		.map((r) => firstNumber(r.values, ids))
		.filter((v): v is number => v !== null);
}

const mean = (values: number[]) =>
	values.length ? round(values.reduce((a, b) => a + b, 0) / values.length) : null;

/**
 * The classroom charts (AP-45): one point a day for the `count` days up to the latest released
 * reading. Air temperature is the day's average with its low and high, rain the day's total
 * (the gauge is the reference), UV the day's highest and soil moisture the day's average.
 * A day without a reading stays empty; it is never filled in.
 */
export function stationCharts(
	records: CoreRecord[],
	releasedThrough: string | null,
	parameters: CoreParameters,
	count = 30
): StationCharts {
	const released =
		releasedThrough === null ? [] : records.filter((r) => r.observedAt <= releasedThrough);
	const latest = released.reduce<string | null>(
		(at, r) => (!at || r.observedAt > at ? r.observedAt : at),
		null
	);
	if (!latest) return { days: [], temperature: [], rain: [], uv: [], soilMoisture: [] };
	const days = daysUpTo(latest.slice(0, 10), count);
	return {
		days,
		temperature: days.map((d) => {
			const values = dayValues(released, d, parameters.airTemperature);
			return values.length
				? { mean: mean(values)!, low: Math.min(...values), high: Math.max(...values) }
				: null;
		}),
		rain: days.map((d) => rainDayOn(released, d, parameters)),
		uv: days.map((d) => {
			const values = dayValues(released, d, parameters.uv);
			return values.length ? Math.max(...values) : null;
		}),
		soilMoisture: days.map((d) => mean(dayValues(released, d, parameters.soilMoisture)))
	};
}
