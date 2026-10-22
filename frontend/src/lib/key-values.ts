import type { DeviceType, Parameter, Reading, ValueType } from '#lib/domain/types.ts';

interface KeyParameter {
	id: string;
	label: string;
	/** Used when the parameter registry has no entry for it. */
	unit?: string;
}

/**
 * The values that matter for each kind of sensor, in display order. A weather station leads
 * with temperature, wind, rain and UV (AP-06); a soil sensor shows soil values and nothing about
 * rain. Other types, and a device that sends none of its type's values, show what it sent.
 */
export const KEY_PARAMETERS: Partial<Record<DeviceType, KeyParameter[]>> = {
	weather_station: [
		{ id: 'air_temperature', label: 'Temperature' },
		{ id: 'wind_avg', label: 'Wind' },
		{ id: 'precip_accum_local_day', label: 'Rain today' },
		{ id: 'uv', label: 'UV index' }
	],
	rain_gauge: [{ id: 'rain_gauge_daily_accumulation', label: 'Rain today' }],
	soil_sensor: [
		{ id: 'soil_moisture', label: 'Soil moisture', unit: '%' },
		{ id: 'soil_temperature', label: 'Soil temperature', unit: '°C' }
	],
	water_probe: [
		{ id: 'water_temperature', label: 'Water temperature', unit: '°C' },
		{ id: 'water_ph', label: 'pH' }
	],
	air_quality_sensor: [
		{ id: 'pm2_5', label: 'PM2.5', unit: 'µg/m³' },
		{ id: 'pm10', label: 'PM10', unit: 'µg/m³' }
	],
	other: []
};

export interface KeyValue {
	id: string;
	label: string;
	unit: string;
	valueType: ValueType;
	value: number | string;
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

/** The key values a reading carries for its device type, labelled and with units. */
export function keyValues(
	type: DeviceType,
	values: Reading['values'],
	parameters: ReadonlyMap<string, Parameter>,
	max = 4
): KeyValue[] {
	const keys = (KEY_PARAMETERS[type] ?? []).filter((k) => values[k.id] !== undefined);
	const chosen: KeyParameter[] = keys.length
		? keys
		: Object.keys(values).map((id) => ({
				id,
				label: parameters.get(id)?.label ?? fallbackLabel(id)
			}));
	return chosen.slice(0, max).map((k) => ({
		id: k.id,
		label: k.label,
		unit: parameters.get(k.id)?.unit ?? k.unit ?? '',
		valueType: parameters.get(k.id)?.valueType ?? 'number',
		value: values[k.id]
	}));
}

/** "Temperature 27°C · Wind 3.5 m/s", for compact lists. */
export function keySummary(
	type: DeviceType,
	values: Reading['values'],
	parameters: ReadonlyMap<string, Parameter>,
	max = 3
) {
	return keyValues(type, values, parameters, max)
		.map((v) => `${v.label} ${formatValue(v.value, v.unit, v.valueType)}`)
		.join(' · ');
}

/** A parameter's label and unit: from the registry, else from the key values above. */
export function describeParameter(id: string, parameters: ReadonlyMap<string, Parameter>) {
	const registered = parameters.get(id);
	if (registered) {
		const { label, unit, valueType = 'number' } = registered;
		return { label, unit, valueType };
	}
	const key = Object.values(KEY_PARAMETERS)
		.flatMap((keys) => keys ?? [])
		.find((k) => k.id === id);
	return {
		label: key?.label ?? fallbackLabel(id),
		unit: key?.unit ?? '',
		valueType: 'number' as ValueType
	};
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
