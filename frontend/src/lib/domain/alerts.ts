import type { DeviceAlert, Reading } from './types';

interface Rule {
	parameterId: string;
	level: DeviceAlert['level'];
	/** True when the value crosses the threshold. */
	test: (value: number) => boolean;
	title: string;
	message: (value: number) => string;
}

/**
 * Default thresholds until classrooms can set their own (AP-08). Ordered so the more severe
 * rule for a parameter comes first; only one alert per parameter is raised.
 */
const RULES: Rule[] = [
	{
		parameterId: 'uv',
		level: 'warning',
		test: (v) => v >= 8,
		title: 'Very high UV',
		message: (v) => `UV index ${v}. Avoid the midday sun and cover up.`
	},
	{
		parameterId: 'uv',
		level: 'caution',
		test: (v) => v >= 6,
		title: 'High UV',
		message: (v) => `UV index ${v}. Use shade and sunscreen.`
	},
	{
		parameterId: 'heat_index',
		level: 'warning',
		test: (v) => v >= 39,
		title: 'Dangerous heat',
		message: (v) => `Feels like ${v}°C. Limit outdoor activity.`
	},
	{
		parameterId: 'air_temperature',
		level: 'caution',
		test: (v) => v >= 32,
		title: 'Hot',
		message: (v) => `${v}°C. Drink water and rest in the shade.`
	},
	{
		parameterId: 'wind_gust',
		level: 'warning',
		test: (v) => v >= 17,
		title: 'Strong gusts',
		message: (v) => `Gusts of ${v} m/s. Secure loose equipment.`
	},
	{
		parameterId: 'precip_accum_local_day',
		level: 'warning',
		test: (v) => v >= 50,
		title: 'Heavy rain',
		message: (v) => `${v} mm today. Watch for flooding.`
	},
	{
		parameterId: 'rain_gauge_daily_accumulation',
		level: 'warning',
		test: (v) => v >= 50,
		title: 'Heavy rain',
		message: (v) => `${v} mm in the gauge today. Watch for flooding.`
	},
	{
		parameterId: 'soil_moisture',
		level: 'caution',
		test: (v) => v <= 15,
		title: 'Dry soil',
		message: (v) => `Soil moisture ${v}%. Plants may need water.`
	}
];

export type ThresholdLevel = 'normal' | DeviceAlert['level'];

/**
 * Where a value sits against the default thresholds, for a badge beside it. Null when the
 * parameter has no threshold.
 */
export function thresholdStatus(
	parameterId: string,
	value: number | string | undefined
): { level: ThresholdLevel; title: string } | null {
	const rules = RULES.filter((r) => r.parameterId === parameterId);
	if (!rules.length || typeof value !== 'number') return null;
	const hit = rules.find((r) => r.test(value));
	return hit ? { level: hit.level, title: hit.title } : { level: 'normal', title: 'Normal' };
}

export function alertsFor(reading: Pick<Reading, 'values'>): DeviceAlert[] {
	const alerts: DeviceAlert[] = [];
	for (const rule of RULES) {
		const value = reading.values[rule.parameterId];
		if (typeof value !== 'number' || !rule.test(value)) continue;
		if (alerts.some((a) => a.parameterId === rule.parameterId)) continue;
		alerts.push({
			parameterId: rule.parameterId,
			level: rule.level,
			title: rule.title,
			message: rule.message(value)
		});
	}
	return alerts;
}
