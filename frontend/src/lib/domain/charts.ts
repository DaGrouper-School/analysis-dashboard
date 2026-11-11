import type { Chart, Parameter } from './types.ts';

/** The chart kinds a number can have (AP-23), in the order the Registry offers them. */
export const CHARTS: Chart[] = ['line', 'line_band', 'daily_bars'];

export const CHART_LABELS: Record<Chart, string> = {
	line: 'Line',
	line_band: 'Line with each day’s low to high',
	daily_bars: 'Daily bars'
};

/** The chart a value gets: its registry choice, a line for other numbers, none for text. */
export function chartOf(parameter: Pick<Parameter, 'chart' | 'valueType'>): Chart | null {
	if ((parameter.valueType ?? 'number') !== 'number') return null;
	return parameter.chart ?? 'line';
}
