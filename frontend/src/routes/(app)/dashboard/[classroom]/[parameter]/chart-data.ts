import type { ChartBars, ChartLine } from '#lib/components/Chart.svelte';
import type { Chart, PublicParameterPage, PublicParameterSeries } from '#lib/domain/types.ts';
import {
	dailyStats,
	inRange,
	rainDays,
	type RangeId
} from '#lib/features/measurements/parameter-page.ts';
import { seriesLabel } from './value-format.ts';

/** The value page's main chart (AP-23): its bars or lines for the chosen range. */

export const SERIES_COLORS = ['var(--color-series-1)', 'var(--color-series-2)'];
export const COMPARE_COLOR = 'var(--color-series-3)';

type RainDay = ReturnType<typeof rainDays>[number];
type DayStats = ReturnType<typeof dailyStats>[number];

/** Rain: one set of bars per source, plus the compared classroom's shown total. */
export function rainBars(
	series: PublicParameterSeries[],
	referenceSourceId: string | null,
	days: RainDay[],
	rangeDays: string[],
	compared: { name: string; days: RainDay[] } | null
): ChartBars[] {
	const bars: ChartBars[] = series.map((s, i) => ({
		id: s.source.id,
		label: seriesLabel(s, referenceSourceId),
		color: SERIES_COLORS[i % SERIES_COLORS.length],
		values: Object.fromEntries(
			days
				.filter((d) => d.totals[s.source.id] !== undefined)
				.map((d) => [d.day, d.totals[s.source.id]])
		)
	}));
	if (compared) {
		bars.push({
			id: 'compare',
			label: compared.name,
			color: COMPARE_COLOR,
			values: Object.fromEntries(
				compared.days
					.filter((d) => d.shown !== null && rangeDays.includes(d.day))
					.map((d) => [d.day, d.shown!])
			)
		});
	}
	return bars;
}

/** The time the chart spans: the last 24 hours, or whole days up to the latest reading. */
export function chartWindow(latest: string | null, range: RangeId, rangeDays: string[]) {
	if (!latest) return { from: 0, to: 0 };
	const to = Date.parse(latest);
	return range === '24h'
		? { from: to - 86_400_000, to }
		: { from: Date.parse(`${rangeDays[0]}T00:00:00.000Z`), to };
}

const noon = (day: string) => Date.parse(`${day}T12:00:00.000Z`);

interface LineContext {
	range: RangeId;
	chart: Chart;
	latest: string | null;
	rangeDays: string[];
	window: { from: number; to: number };
}

/**
 * One source as a line: every reading for the last 24 hours or a plain line chart, otherwise
 * each day's average with its low to high as a band (none for a dashed comparison line).
 */
export function seriesLine(
	s: PublicParameterSeries,
	dayStats: DayStats[],
	base: Omit<ChartLine, 'points' | 'band'>,
	{ range, chart, latest, rangeDays, window }: LineContext
): ChartLine {
	const numbers = s.points.filter(
		(p): p is { observedAt: string; value: number } => typeof p.value === 'number'
	);
	if (range === '24h' || chart === 'line') {
		const points = latest ? inRange(numbers, latest, range) : [];
		return {
			...base,
			points: points
				.map((p) => ({ at: Date.parse(p.observedAt), value: p.value }))
				.filter((p) => p.at >= window.from)
		};
	}
	const days = dayStats.filter((d) => rangeDays.includes(d.day));
	return {
		...base,
		points: days.map((d) => ({ at: noon(d.day), value: d.mean })),
		band: base.dashed ? undefined : days.map((d) => ({ at: noon(d.day), low: d.low, high: d.high }))
	};
}

/** The other classroom's line or bars: its reference gauge, or its first source. */
export function comparedSeriesOf(page: PublicParameterPage) {
	return page.series.find((s) => s.source.id === page.rain?.referenceSourceId) ?? page.series[0];
}

/** A legend entry for each series when there is more than one to tell apart. */
export function legendOf(rain: boolean, bars: ChartBars[], lines: ChartLine[]) {
	if (rain) return bars.map((b) => ({ label: b.label, color: b.color, dashed: false }));
	return lines.length > 1
		? lines.map((l) => ({ label: l.label, color: l.color, dashed: !!l.dashed }))
		: [];
}
