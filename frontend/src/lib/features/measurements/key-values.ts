import type { Parameter, Reading, ValueType } from '#lib/domain/types.ts';

export interface KeyValue {
	id: string;
	label: string;
	unit: string;
	valueType: ValueType;
	value: number | string;
	icon?: string;
}

// The browser's own language and time zone.
const moment = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

/**
 * A value with its unit, rounded for reading at a glance. A time (Unix seconds) shows as a date
 * in the reader's language and time zone. Missing values show a dash.
 */
export function formatValue(
	value: number | string | undefined,
	unit = '',
	valueType: ValueType = 'number'
) {
	if (value === undefined) return '—';
	if (typeof value === 'string') return value;
	if (valueType === 'time') return moment.format(new Date(value * 1000));
	const rounded = Math.abs(value) >= 100 ? Math.round(value) : Math.round(value * 10) / 10;
	return `${rounded}${unit && !unit.startsWith('°') ? ' ' : ''}${unit}`;
}

function fallbackLabel(id: string) {
	return id.replaceAll('_', ' ').replace(/^./, (c) => c.toUpperCase());
}

/**
 * The values a reading leads with (AP-45): `ids` from its device type in the Registry (its tiles
 * or its cards), in the order a platform admin set there. With `fallback`, a type with none set,
 * or a reading with none of them, shows what it sent; without, it shows nothing. Labels and units come from the registry; each value's icon is its
 * domain's, when `domainIcons` is given.
 */
export function keyValues(
	ids: readonly string[] | undefined,
	values: Reading['values'],
	parameters: ReadonlyMap<string, Parameter>,
	{
		max = Infinity,
		domainIcons,
		fallback = true
	}: { max?: number; domainIcons?: ReadonlyMap<string, string>; fallback?: boolean } = {}
): KeyValue[] {
	const set = (ids ?? []).filter((id) => values[id] !== undefined);
	const chosen = set.length || !fallback ? set : Object.keys(values);
	return chosen.slice(0, max).map((id) => {
		const param = parameters.get(id);
		return {
			id,
			label: param?.label ?? fallbackLabel(id),
			unit: param?.unit ?? '',
			valueType: param?.valueType ?? 'number',
			value: values[id],
			icon: param?.domain ? domainIcons?.get(param.domain) : undefined
		};
	});
}

/** "Air temperature 27°C · Average wind speed 3.5 m/s", for compact lists. */
export function keySummary(
	ids: readonly string[] | undefined,
	values: Reading['values'],
	parameters: ReadonlyMap<string, Parameter>,
	max = 3
) {
	return keyValues(ids, values, parameters, { max })
		.map((v) => `${v.label} ${formatValue(v.value, v.unit, v.valueType)}`)
		.join(' · ');
}

/** A parameter's label and unit from the registry, or its ID made readable. */
export function describeParameter(id: string, parameters: ReadonlyMap<string, Parameter>) {
	const registered = parameters.get(id);
	if (registered) {
		const { label, unit, valueType = 'number' } = registered;
		return { label, unit, valueType };
	}
	return { label: fallbackLabel(id), unit: '', valueType: 'number' as ValueType };
}

const relative = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });

/** `3 hours ago`, `yesterday`: how long before `now` a time (Unix seconds) was, in the reader's language. */
export function timeAgo(seconds: number, now = Date.now()) {
	const ms = seconds * 1000 - now;
	const units: [number, Intl.RelativeTimeFormatUnit][] = [
		[86_400_000, 'day'],
		[3_600_000, 'hour'],
		[60_000, 'minute']
	];
	for (const [size, unit] of units) {
		if (Math.abs(ms) >= size) return relative.format(Math.round(ms / size), unit);
	}
	return relative.format(0, 'minute');
}
