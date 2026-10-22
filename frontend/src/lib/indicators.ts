import type { Parameter, Reading } from '#lib/domain/types.ts';
import { formatValue } from '#lib/key-values.ts';

/**
 * Indicators (AP-07): where a value sits among its registry bands, and which way it is heading.
 * Each describes one value on its own; nothing here compares values or says why one changed.
 */

/** The registry's level word for a value, or null when the parameter has no bands. */
export function bandFor(parameter: Parameter | undefined, value: number | string | undefined) {
	if (!parameter?.bands?.length || typeof value !== 'number') return null;
	return parameter.bands.find((b) => b.upTo === null || value < b.upTo)?.label ?? null;
}

export type TrendDirection = 'rising' | 'steady' | 'falling';

export interface Trend {
	direction: TrendDirection;
	/** The change across the readings along the fitted line, in the parameter's unit. */
	change: number;
	/** First and last reading times, in ms. */
	from: number;
	to: number;
}

export const TREND_LABELS: Record<TrendDirection, string> = {
	rising: 'Rising',
	steady: 'Steady',
	falling: 'Falling'
};

/**
 * The trend of one value over the readings: a least-squares line through them, and its change
 * from the first reading to the last. A change within the parameter's noise reads as steady.
 * Null with fewer than two numeric readings.
 */
export function trendOf(
	readings: Pick<Reading, 'observedAt' | 'values'>[],
	parameter: Pick<Parameter, 'id' | 'noise'>
): Trend | null {
	const points = readings
		.map((r) => ({ t: Date.parse(r.observedAt), v: r.values[parameter.id] }))
		.filter((p): p is { t: number; v: number } => typeof p.v === 'number')
		.sort((a, b) => a.t - b.t);
	if (points.length < 2) return null;
	const from = points[0].t;
	const to = points.at(-1)!.t;
	const meanT = points.reduce((n, p) => n + p.t, 0) / points.length;
	const meanV = points.reduce((n, p) => n + p.v, 0) / points.length;
	const spread = points.reduce((n, p) => n + (p.t - meanT) ** 2, 0);
	const slope = spread ? points.reduce((n, p) => n + (p.t - meanT) * (p.v - meanV), 0) / spread : 0;
	const change = slope * (to - from);
	// Rounded as it is shown, so "up 0" never reads as rising.
	const shown = Math.abs(change) >= 100 ? Math.round(change) : Math.round(change * 10) / 10;
	const direction =
		Math.abs(change) <= (parameter.noise ?? 0) || shown === 0
			? 'steady'
			: change > 0
				? 'rising'
				: 'falling';
	return { direction, change, from, to };
}

/** `3 days`, `14 hours`, `40 minutes`: the span a trend covers. */
export function spanText(ms: number) {
	const units: [number, string][] = [
		[86_400_000, 'day'],
		[3_600_000, 'hour'],
		[60_000, 'minute']
	];
	for (const [size, name] of units) {
		const n = Math.round(ms / size);
		if (n >= 1 && (ms >= size || name === 'minute')) return `${n} ${name}${n === 1 ? '' : 's'}`;
	}
	return 'under a minute';
}

/** `Rising, up 2.1 °C over 7 days`, or `Steady over 3 days`. */
export function trendText(trend: Trend, unit: string) {
	const over = `over ${spanText(trend.to - trend.from)}`;
	if (trend.direction === 'steady') return `Steady ${over}`;
	const by = formatValue(Math.abs(trend.change), unit);
	return `${TREND_LABELS[trend.direction]}, ${trend.direction === 'rising' ? 'up' : 'down'} ${by} ${over}`;
}
