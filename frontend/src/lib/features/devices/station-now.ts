import type {
	CoreReadings,
	DeviceSummaryCard,
	NowTile,
	NowTileConfig,
	Parameter,
	PublicValue
} from '#lib/domain/types.ts';
import { formatValue, type KeyValue } from '#lib/features/measurements/key-values.ts';
import { recentTrend } from '#lib/features/measurements/indicators.ts';

/** The 16 compass points, clockwise from north, in words. */
const POINTS = [
	'north',
	'north-northeast',
	'northeast',
	'east-northeast',
	'east',
	'east-southeast',
	'southeast',
	'south-southeast',
	'south',
	'south-southwest',
	'southwest',
	'west-southwest',
	'west',
	'west-northwest',
	'northwest',
	'north-northwest'
];

/** A direction in degrees as a compass point in words: 112° is "east-southeast". */
export function compassPoint(degrees: number): string {
	const i = Math.round((((degrees % 360) + 360) % 360) / 22.5) % 16;
	return POINTS[i];
}

/**
 * A short place name for a location tab (AP-43): the last part of the lab's place and the
 * country, so "Trinity School, Addis Ababa" in Ethiopia reads "Addis Ababa, Ethiopia".
 */
export function placeLabel(placeName: string | null, country: string | null): string {
	const town = placeName?.split(',').at(-1)?.trim() || null;
	return [town, country].filter((part, i, all) => part && all.indexOf(part) === i).join(', ');
}

/** Up to two initials for a photo placeholder: "Bay Street School" is "BS". */
export function initials(name: string): string {
	const words = name.split(/[\s,]+/).filter((w) => /^\p{Lu}/u.test(w));
	return (words.length ? words : name.split(/\s+/))
		.slice(0, 2)
		.map((w) => w[0]?.toUpperCase() ?? '')
		.join('');
}

/** A rounded position as "9.03° N, 38.74° E". */
export function formatCoordinates(location: { lat: number; lng: number }): string {
	const part = (value: number, positive: string, negative: string) =>
		`${Math.abs(value).toFixed(2)}° ${value >= 0 ? positive : negative}`;
	return `${part(location.lat, 'N', 'S')}, ${part(location.lng, 'E', 'W')}`;
}

/**
 * "What's happening now?" tiles (AP-43) from the latest released values: per tile, the first
 * configured parameter with a value, the newest across sources, or the classroom's core reading
 * when the tile names one. Missing data stays null.
 */
export function nowTiles(
	config: NowTileConfig[],
	values: PublicValue[],
	parameters: Parameter[],
	core: CoreReadings | null = null
): NowTile[] {
	const latest = (ids: string[]) => {
		for (const id of ids) {
			const found = values
				.filter((v) => v.parameterId === id)
				.sort((a, b) => b.observedAt.localeCompare(a.observedAt))[0];
			if (found) return found;
		}
		return null;
	};
	const paramOf = (id: string) => parameters.find((p) => p.id === id);
	return config.map((tile) => {
		if (tile.core && core) {
			const value = core[tile.core.value];
			const unit = paramOf(tile.parameterIds[0])?.unit ?? '';
			const week = tile.core.caption ? core[tile.core.caption.value] : null;
			let caption: string | null = null;
			if (value === 0 && tile.zeroText) caption = tile.zeroText;
			if (tile.core.caption && week !== null) {
				const text = tile.core.caption.text.replace('{value}', formatValue(week, unit));
				caption = caption ? `${caption} · ${text}` : text;
			}
			return {
				id: tile.id,
				label: tile.label,
				icon: tile.icon,
				parameterId: null,
				value,
				unit,
				caption: value === null ? null : caption,
				observedAt: value === null ? null : core.lastSync,
				last24Hours: [],
				trend: null
			};
		}
		const shown = latest(tile.parameterIds);
		const param = shown ? paramOf(shown.parameterId) : paramOf(tile.parameterIds[0]);
		let caption: string | null = null;
		if (shown && tile.zeroText && shown.value === 0) caption = tile.zeroText;
		else if (shown && tile.caption) {
			const extra = latest(tile.caption.parameterIds);
			if (extra) {
				const extraParam = paramOf(extra.parameterId);
				const text =
					tile.caption.format === 'compass' && typeof extra.value === 'number'
						? compassPoint(extra.value)
						: formatValue(extra.value, extraParam?.unit, extraParam?.valueType);
				caption = tile.caption.text.replace('{value}', text);
			}
		}
		return {
			id: tile.id,
			label: tile.label,
			icon: tile.icon,
			parameterId: shown?.parameterId ?? null,
			value: shown?.value ?? null,
			unit: param?.unit ?? '',
			caption,
			observedAt: shown?.observedAt ?? null,
			last24Hours: [],
			trend: null
		};
	});
}

const HOUR = 3_600_000;

/**
 * A value's last 24 hours up to `until`, one point an hour, oldest first (AP-43): the latest
 * reading in each hour from the source the tile shows. An hour without a reading stays empty.
 */
export function last24Hours(
	records: { observedAt: string; values: Record<string, unknown> }[],
	parameterIds: string[],
	until: string
): (number | null)[] {
	const end = Date.parse(until);
	const hours: ({ at: number; value: number } | null)[] = Array(24).fill(null);
	for (const record of records) {
		const at = Date.parse(record.observedAt);
		if (at > end || at <= end - 24 * HOUR) continue;
		const value = parameterIds.map((id) => record.values[id]).find((v) => typeof v === 'number');
		if (typeof value !== 'number') continue;
		const i = 23 - Math.floor((end - at) / HOUR);
		if (!hours[i] || at > hours[i]!.at) hours[i] = { at, value };
	}
	return hours.map((h) => h?.value ?? null);
}

/**
 * "What's happening now?" tiles for one device (AP-45): its key values from its newest reading,
 * each with its last 24 hours and which way it went. `readings` are the device's own, newest first.
 */
export function readingTiles(
	readings: { observedAt: string; values: Record<string, unknown> }[],
	values: KeyValue[],
	icon: string,
	parameters: ReadonlyMap<string, Parameter>
): NowTile[] {
	const at = readings[0]?.observedAt ?? null;
	return values.map((v) => {
		const parameter = parameters.get(v.id);
		const numeric = !!at && typeof v.value === 'number';
		return {
			id: v.id,
			label: v.label,
			icon: v.icon ?? icon,
			parameterId: v.id,
			value: v.value ?? null,
			unit: v.unit,
			caption: null,
			observedAt: at,
			last24Hours: numeric ? last24Hours(readings, [v.id], at!) : [],
			trend: numeric && parameter ? recentTrend(readings, parameter, at!) : null
		};
	});
}

/**
 * A device's card numbers over the 7 days up to its newest reading (AP-45), for the cards
 * under "What's happening now?". `readings` are the device's own, newest first.
 */
export function deviceSummary(
	readings: { observedAt: string; values: Record<string, unknown> }[],
	values: KeyValue[],
	max = Infinity
): DeviceSummaryCard[] {
	const newest = readings[0];
	if (!newest) return [];
	const today = newest.observedAt.slice(0, 10);
	const end = Date.parse(`${today}T00:00:00.000Z`);
	const days = Array.from({ length: 7 }, (_, i) =>
		new Date(end - (6 - i) * 86_400_000).toISOString().slice(0, 10)
	);
	const numberOf = (r: (typeof readings)[number], id: string) =>
		typeof r.values[id] === 'number' ? (r.values[id] as number) : null;
	const range = (list: number[]) =>
		list.length ? { low: Math.min(...list), high: Math.max(...list) } : { low: null, high: null };
	return values
		.filter((v) => typeof v.value === 'number')
		.slice(0, max)
		.map((v) => {
			const inWeek = readings.filter((r) => r.observedAt.slice(0, 10) >= days[0]);
			const numbers = (day?: string) =>
				inWeek
					.filter((r) => !day || r.observedAt.startsWith(day))
					.map((r) => numberOf(r, v.id))
					.filter((n): n is number => n !== null);
			const todays = range(numbers(today));
			const weeks = range(numbers());
			return {
				id: v.id,
				label: v.label,
				unit: v.unit,
				icon: v.icon ?? null,
				latest: numberOf(newest, v.id),
				observedAt: newest.observedAt,
				// The last reading of each day: a day's total for an accumulation, else its close.
				week: days.map((day) => ({ day, value: numbers(day)[0] ?? null })),
				todayLow: todays.low,
				todayHigh: todays.high,
				weekLow: weeks.low,
				weekHigh: weeks.high
			};
		});
}

/** The device a page opens on: the one asked for, else a weather station, else the first. */
export function pickDevice<T extends { id: string; type: string }>(
	devices: T[],
	id: string | null | undefined
): T | null {
	return (
		devices.find((d) => d.id === id) ??
		devices.find((d) => d.type === 'weather_station') ??
		devices[0] ??
		null
	);
}

/** How long ago, in plain words: "5 mins ago", "3 days ago". */
export function timeAgo(iso: string, now: number = Date.now()): string {
	const minutes = Math.max(0, Math.round((now - Date.parse(iso)) / 60_000));
	if (minutes < 1) return 'just now';
	if (minutes < 60) return `${minutes} ${minutes === 1 ? 'min' : 'mins'} ago`;
	const hours = Math.round(minutes / 60);
	if (hours < 24) return `${hours} ${hours === 1 ? 'hour' : 'hours'} ago`;
	const days = Math.round(hours / 24);
	return `${days} ${days === 1 ? 'day' : 'days'} ago`;
}
