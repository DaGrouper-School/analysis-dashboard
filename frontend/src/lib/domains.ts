import type { EnvironmentalDomain, Parameter, Reading } from '#lib/domain/types.ts';

/** The registry's environmental domains, in the order the dashboard shows them. */
export const DOMAINS: { id: EnvironmentalDomain; label: string; icon: string }[] = [
	{ id: 'AIR', label: 'Air', icon: '🌳' },
	{ id: 'RAIN', label: 'Rain', icon: '🌧️' },
	{ id: 'WIND', label: 'Wind', icon: '💨' },
	{ id: 'SUN', label: 'Sun', icon: '☀️' },
	{ id: 'WAVES', label: 'Waves', icon: '🌊' },
	{ id: 'LIGHTNING', label: 'Lightning', icon: '⚡️' }
];

/** Values outside the registry, such as soil moisture, sit under their own tab. */
export const OTHER_TAB = { id: 'OTHER', label: 'Sensor values', icon: '📟' } as const;

export type DomainTabId = EnvironmentalDomain | typeof OTHER_TAB.id;

export interface DomainTab {
	id: DomainTabId;
	label: string;
	icon: string;
	/** Numeric parameters the device sends in this domain, in registry order. */
	parameterIds: string[];
}

/**
 * The domains a device's readings cover, as tabs. Only numeric values count: a trend needs
 * numbers. Domains the device sends nothing for are left out.
 */
export function domainTabs(readings: Reading[], parameters: Parameter[]): DomainTab[] {
	const sent = new Set<string>();
	for (const r of readings)
		for (const [id, value] of Object.entries(r.values)) if (typeof value === 'number') sent.add(id);
	const registered = new Set(parameters.map((p) => p.id));
	const tabs: DomainTab[] = DOMAINS.map((d) => ({
		...d,
		parameterIds: parameters.filter((p) => p.domain === d.id && sent.has(p.id)).map((p) => p.id)
	}));
	const other = [...sent].filter((id) => !registered.has(id));
	if (other.length) tabs.push({ ...OTHER_TAB, parameterIds: other });
	return tabs.filter((t) => t.parameterIds.length);
}

export const TIME_RANGES = [
	{ id: '24h', label: '24 h', hours: 24 },
	{ id: '7d', label: '7 d', hours: 7 * 24 },
	{ id: '30d', label: '30 d', hours: 30 * 24 }
] as const;

export type TimeRange = (typeof TIME_RANGES)[number]['id'];

export function isTimeRange(value: string | null): value is TimeRange {
	return TIME_RANGES.some((r) => r.id === value);
}

/**
 * Readings within the range, counted back from the newest reading so a quiet sensor still shows
 * its last stretch of data rather than an empty chart.
 */
export function withinRange(readings: Reading[], range: TimeRange): Reading[] {
	if (!readings.length) return [];
	const hours = TIME_RANGES.find((r) => r.id === range)!.hours;
	const newest = Math.max(...readings.map((r) => Date.parse(r.observedAt)));
	return readings.filter((r) => Date.parse(r.observedAt) > newest - hours * 3_600_000);
}
