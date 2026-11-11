import type {
	Parameter,
	Chart,
	PublicParameterPage,
	PublicParameterSeries
} from '#lib/domain/types.ts';
import { csvField } from '#lib/raw-data.ts';

/** The time ranges of a value's main chart (AP-23), newest data last. */
export type RangeId = '24h' | '7d' | '30d' | 'season';

const DAY = 86_400_000;

export const RANGES: { id: RangeId; label: string; days: number }[] = [
	{ id: '24h', label: '24h', days: 1 },
	{ id: '7d', label: '7d', days: 7 },
	{ id: '30d', label: '30d', days: 30 },
	// A season is three months: long enough for the rains, short enough to read day by day.
	{ id: 'season', label: 'Season', days: 91 }
];

/** The chart a value gets: its registry choice, a line for other numbers, none for text. */
export function chartOf(parameter: Pick<Parameter, 'chart' | 'valueType'>): Chart | null {
	if ((parameter.valueType ?? 'number') !== 'number') return null;
	return parameter.chart ?? 'line';
}

/** UTC day of an ISO time, like the core readings, so a day means the same everywhere. */
export const dayOf = (iso: string) => iso.slice(0, 10);

/** The days of a range ending on `lastDay`, oldest first. */
export function daysOf(lastDay: string, days: number): string[] {
	const end = Date.parse(`${lastDay}T00:00:00.000Z`);
	return Array.from({ length: days }, (_, i) =>
		new Date(end - (days - 1 - i) * DAY).toISOString().slice(0, 10)
	);
}

/** The latest observation time across the series, or null with none. */
export function latestOf(series: Pick<PublicParameterSeries, 'points'>[]): string | null {
	return series
		.flatMap((s) => s.points.map((p) => p.observedAt))
		.reduce<string | null>((latest, at) => (!latest || at > latest ? at : latest), null);
}

const round = (value: number) => Math.round(value * 10) / 10;

interface DayStats {
	day: string;
	low: number;
	high: number;
	mean: number;
	count: number;
}

/** Each day's low, high, mean and number of readings, oldest first. Days without readings are left out. */
export function dailyStats(points: PublicParameterSeries['points']): DayStats[] {
	const byDay = new Map<string, number[]>();
	for (const point of points) {
		if (typeof point.value !== 'number') continue;
		const day = dayOf(point.observedAt);
		byDay.set(day, [...(byDay.get(day) ?? []), point.value]);
	}
	return [...byDay.entries()]
		.sort(([a], [b]) => a.localeCompare(b))
		.map(([day, values]) => ({
			day,
			low: Math.min(...values),
			high: Math.max(...values),
			mean: round(values.reduce((a, b) => a + b, 0) / values.length),
			count: values.length
		}));
}

interface RainDay {
	day: string;
	/** Each source's total for the day, by source ID; a source with no reading is missing. */
	totals: Record<string, number>;
	/**
	 * The day's rain as shown: the manual gauge's total, or, without one that day, the first
	 * other source's. Never an average of sources.
	 */
	shown: number | null;
	/** Which source `shown` comes from. */
	shownFrom: string | null;
	/** How far the sources are apart; null with one source. */
	spread: number | null;
	/** The sources differ by more than the protocol allows, so the day goes to review. */
	review: boolean;
}

/**
 * Rain day by day (AP-23): each source's total is its highest running total that day. The
 * manual gauge is the reference; when any source is further from it than `tolerance`, the day
 * is flagged for review and still shows the gauge's total, never an average.
 */
export function rainDays(
	series: Pick<PublicParameterSeries, 'source' | 'points'>[],
	referenceSourceId: string | null,
	tolerance: number
): RainDay[] {
	const days = new Map<string, Record<string, number>>();
	for (const { source, points } of series) {
		for (const point of points) {
			if (typeof point.value !== 'number') continue;
			const day = dayOf(point.observedAt);
			const totals = days.get(day) ?? {};
			totals[source.id] = Math.max(totals[source.id] ?? 0, point.value);
			days.set(day, totals);
		}
	}
	const order = [
		...(referenceSourceId ? [referenceSourceId] : []),
		...series.map((s) => s.source.id).filter((id) => id !== referenceSourceId)
	];
	return [...days.entries()]
		.sort(([a], [b]) => a.localeCompare(b))
		.map(([day, totals]) => {
			const shownFrom = order.find((id) => totals[id] !== undefined) ?? null;
			const shown = shownFrom === null ? null : totals[shownFrom];
			const values = Object.values(totals);
			const spread = values.length > 1 ? round(Math.max(...values) - Math.min(...values)) : null;
			return {
				day,
				totals,
				shown,
				shownFrom,
				spread,
				review: shown !== null && values.some((v) => Math.abs(v - shown) > tolerance)
			};
		});
}

/** Readings inside a range that ends at `latest`: the last 24 hours, or whole days. */
export function inRange<T extends { observedAt: string }>(
	points: T[],
	latest: string,
	range: RangeId
): T[] {
	const { days } = RANGES.find((r) => r.id === range)!;
	if (range === '24h') {
		const from = Date.parse(latest) - DAY;
		return points.filter((p) => Date.parse(p.observedAt) > from);
	}
	const first = daysOf(dayOf(latest), days)[0];
	return points.filter((p) => dayOf(p.observedAt) >= first);
}

const PARAMETER_CSV_COLUMNS = [
	'observed_at',
	'classroom_id',
	'source',
	'parameter_id',
	'value',
	'unit',
	'licence',
	'licence_url',
	'attribution',
	'restrictions'
] as const;

/**
 * The page's released values as CSV, one row per reading, oldest first (AP-23 download). Like
 * the members' export, every row carries the classroom's licence and attribution.
 */
export function parameterCsv(
	classroomId: string,
	series: PublicParameterSeries[],
	units: Record<string, string>,
	terms: PublicParameterPage['terms']
): string {
	const restrictions = terms.restrictions.join(' ');
	const rows = series
		.flatMap((s) =>
			s.points.map((p) => ({
				observedAt: p.observedAt,
				source: s.source.name ?? 'Field observations',
				parameterId: s.parameterId,
				value: p.value
			}))
		)
		.sort((a, b) => a.observedAt.localeCompare(b.observedAt) || a.source.localeCompare(b.source));
	return (
		[
			PARAMETER_CSV_COLUMNS.join(','),
			...rows.map((r) =>
				[
					r.observedAt,
					classroomId,
					r.source,
					r.parameterId,
					r.value,
					units[r.parameterId] ?? '',
					terms.licence,
					terms.licenceUrl,
					terms.attribution,
					restrictions
				]
					.map((v) => csvField(v))
					.join(',')
			)
		].join('\r\n') + '\r\n'
	);
}
