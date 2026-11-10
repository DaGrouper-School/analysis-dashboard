import { describe, expect, it } from 'vitest';
import type { PublicParameterSeries } from '#lib/domain/types.ts';
import {
	chartOf,
	dailyStats,
	daysOf,
	inRange,
	latestOf,
	parameterCsv,
	rainDays
} from './parameter-page';

const series = (
	id: string,
	parameterId: string,
	points: [string, number][]
): PublicParameterSeries => ({
	source: { id, name: id === 'field' ? null : id, kind: id === 'field' ? 'field' : 'device' },
	parameterId,
	points: points.map(([observedAt, value]) => ({ observedAt, value }))
});

describe('parameter page (AP-23)', () => {
	it('charts a value the way the registry says, and never charts text', () => {
		expect(chartOf({ chart: 'daily_bars' })).toBe('daily_bars');
		expect(chartOf({})).toBe('line');
		expect(chartOf({ valueType: 'text', chart: 'line' })).toBeNull();
	});

	it('sums up each day with its low and high', () => {
		expect(
			dailyStats([
				{ observedAt: '2026-10-19T06:00:00Z', value: 11 },
				{ observedAt: '2026-10-19T14:00:00Z', value: 24 },
				{ observedAt: '2026-10-19T20:00:00Z', value: 16 },
				{ observedAt: '2026-10-20T06:00:00Z', value: 12 }
			])
		).toEqual([
			{ day: '2026-10-19', low: 11, high: 24, mean: 17, count: 3 },
			{ day: '2026-10-20', low: 12, high: 12, mean: 12, count: 1 }
		]);
	});

	it('shows the gauge’s rain and flags a day the sources disagree, never averaging', () => {
		const days = rainDays(
			[
				series('station', 'precip_accum_local_day', [
					['2026-10-19T09:00:00Z', 3],
					['2026-10-19T18:00:00Z', 8.5],
					['2026-10-20T09:00:00Z', 4],
					['2026-10-21T09:00:00Z', 2]
				]),
				series('gauge', 'rain_gauge_daily_accumulation', [
					['2026-10-19T06:00:00Z', 4],
					['2026-10-20T06:00:00Z', 5]
				])
			],
			'gauge',
			2
		);
		expect(days).toEqual([
			{
				day: '2026-10-19',
				totals: { station: 8.5, gauge: 4 },
				shown: 4,
				shownFrom: 'gauge',
				spread: 4.5,
				review: true
			},
			{
				day: '2026-10-20',
				totals: { station: 4, gauge: 5 },
				shown: 5,
				shownFrom: 'gauge',
				spread: 1,
				review: false
			},
			// No gauge reading: the station's total, on its own, with nothing to check against.
			{
				day: '2026-10-21',
				totals: { station: 2 },
				shown: 2,
				shownFrom: 'station',
				spread: null,
				review: false
			}
		]);
	});

	it('keeps a range to the last 24 hours or whole days', () => {
		const points = [
			{ observedAt: '2026-10-13T23:00:00Z' },
			{ observedAt: '2026-10-14T00:00:00Z' },
			{ observedAt: '2026-10-19T07:00:00Z' },
			{ observedAt: '2026-10-20T08:00:00Z' }
		];
		const latest = latestOf([series('a', 'x', [['2026-10-20T08:00:00Z', 1]])])!;
		expect(inRange(points, latest, '24h')).toEqual(points.slice(3));
		expect(inRange(points, latest, '7d')).toEqual(points.slice(1));
		expect(daysOf('2026-10-20', 3)).toEqual(['2026-10-18', '2026-10-19', '2026-10-20']);
	});

	it('downloads every released reading with the classroom’s licence on each row', () => {
		const csv = parameterCsv(
			'cls_trinity',
			[
				series('gauge', 'rain_gauge_daily_accumulation', [['2026-10-20T06:00:00Z', 5]]),
				series('field', 'rain_gauge_daily_accumulation', [['2026-10-19T06:00:00Z', 4]])
			],
			{ rain_gauge_daily_accumulation: 'mm' },
			{ licence: 'CC BY 4.0', licenceUrl: null, attribution: 'Trinity, 2026', restrictions: [] }
		);
		expect(csv.split('\r\n')).toEqual([
			'observed_at,classroom_id,source,parameter_id,value,unit,licence,licence_url,attribution,restrictions',
			'2026-10-19T06:00:00Z,cls_trinity,Field observations,rain_gauge_daily_accumulation,4,mm,CC BY 4.0,,"Trinity, 2026",',
			'2026-10-20T06:00:00Z,cls_trinity,gauge,rain_gauge_daily_accumulation,5,mm,CC BY 4.0,,"Trinity, 2026",',
			''
		]);
	});
});
