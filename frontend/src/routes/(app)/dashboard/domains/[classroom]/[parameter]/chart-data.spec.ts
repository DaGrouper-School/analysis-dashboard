import { describe, expect, it } from 'vitest';
import type { PublicParameterSeries } from '#lib/domain/types.ts';
import { chartWindow, legendOf, rainBars, seriesLine } from './chart-data.ts';

const source = (id: string, name: string) =>
	({ source: { id, name }, parameterId: 'rain', points: [] }) as unknown as PublicParameterSeries;

describe('value page chart data', () => {
	it('spans the last 24 hours, or whole days up to the latest reading', () => {
		const latest = '2026-10-20T06:00:00.000Z';
		expect(chartWindow(latest, '24h', ['2026-10-20'])).toEqual({
			from: Date.parse(latest) - 86_400_000,
			to: Date.parse(latest)
		});
		expect(chartWindow(latest, '7d', ['2026-10-14', '2026-10-20']).from).toBe(
			Date.parse('2026-10-14T00:00:00.000Z')
		);
		expect(chartWindow(null, '7d', [])).toEqual({ from: 0, to: 0 });
	});

	it('draws rain as bars per source, the gauge marked, and the compared classroom after', () => {
		const days = [
			{ day: '2026-10-19', totals: { gauge: 4, station: 5 }, shown: 4 },
			{ day: '2026-10-20', totals: { gauge: 6 }, shown: 6 }
		] as never[];
		const bars = rainBars(
			[source('gauge', 'Rain Gauge'), source('station', 'Station')],
			'gauge',
			days,
			['2026-10-19', '2026-10-20'],
			{ name: 'Nassau', days: [{ day: '2026-10-20', shown: 2 }] as never[] }
		);
		expect(bars.map((b) => [b.label, b.values])).toEqual([
			['Rain Gauge (reference)', { '2026-10-19': 4, '2026-10-20': 6 }],
			['Station', { '2026-10-19': 5 }],
			['Nassau', { '2026-10-20': 2 }]
		]);
		expect(legendOf(true, bars, [])).toHaveLength(3);
	});

	it('draws a band of each day’s low to high, but not under a dashed comparison', () => {
		const stats = [{ day: '2026-10-20', low: 14, mean: 18, high: 24, count: 3 }] as never[];
		const context = {
			range: '7d' as const,
			chart: 'line_band' as const,
			latest: '2026-10-20T06:00:00.000Z',
			rangeDays: ['2026-10-20'],
			window: { from: 0, to: 1 }
		};
		const line = seriesLine(source('s', 'S'), stats, { id: 's', label: 'S', color: 'c' }, context);
		expect(line.points[0].value).toBe(18);
		expect(line.band?.[0]).toMatchObject({ low: 14, high: 24 });
		const dashed = seriesLine(
			source('s', 'S'),
			stats,
			{ id: 'x', label: 'X', color: 'c', dashed: true },
			context
		);
		expect(dashed.band).toBeUndefined();
		expect(legendOf(false, [], [line])).toEqual([]);
	});
});
