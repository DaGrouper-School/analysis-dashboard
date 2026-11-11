import type { Parameter, PublicParameterSeries } from '#lib/domain/types.ts';
import { formatValue } from '#lib/features/measurements/key-values.ts';

/** Formatting shared by the value page's sections. */

export const dateTime = new Intl.DateTimeFormat(undefined, {
	dateStyle: 'medium',
	timeStyle: 'short'
});

const dayLong = new Intl.DateTimeFormat(undefined, {
	weekday: 'short',
	day: 'numeric',
	month: 'short',
	timeZone: 'UTC'
});

/** `Tue 20 Oct` for a UTC day such as `2026-10-20`. */
export const dayText = (day: string) => dayLong.format(Date.parse(`${day}T12:00:00.000Z`));

/** A value in the parameter's unit, or a dash when there is none. */
export const showValue = (
	value: number | string | null | undefined,
	parameter: Pick<Parameter, 'unit' | 'valueType'>
) =>
	value === null || value === undefined
		? '–'
		: formatValue(value, parameter.unit, parameter.valueType);

export const sourceName = (s: Pick<PublicParameterSeries, 'source'>) =>
	s.source.name ?? 'Field observations';

/** The source's name, marked when it is the classroom's reference gauge. */
export const seriesLabel = (s: PublicParameterSeries, referenceSourceId: string | null) =>
	referenceSourceId === s.source.id ? `${sourceName(s)} (reference)` : sourceName(s);
