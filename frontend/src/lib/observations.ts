import type {
	DeviceTypeDefinition,
	ObservationTemplate,
	Parameter,
	TemplateField
} from '#lib/domain/types.ts';

/**
 * Manual observations (AP-16): templates, and the checks that run before an observation is sent.
 * The form and the backend run the same checks, the form on what was typed, the backend on the
 * values it receives.
 */

/**
 * The registry's templates, plus a plain one for each device type that has none, so a device
 * type a platform admin registers can be recorded by hand straight away.
 */
export function withDeviceTemplates(
	templates: ObservationTemplate[],
	deviceTypes: DeviceTypeDefinition[],
	parameters: Parameter[]
): ObservationTemplate[] {
	const covered = new Set(templates.map((t) => t.deviceType));
	const derived = deviceTypes
		.filter((type) => !covered.has(type.id) && type.parameterIds.length)
		.map((type): ObservationTemplate => {
			const fields = type.parameterIds.map((parameterId) => ({ parameterId, required: false }));
			// At least one value is needed; the first one the device sends stands for the reading.
			fields[0].required = true;
			const domains = type.parameterIds.map(
				(id) => parameters.find((p) => p.id === id)?.domain ?? null
			);
			return {
				id: `device:${type.id}`,
				label: `${type.label} reading`,
				description: type.description,
				domain: domains.every((d) => d === domains[0]) ? domains[0] : null,
				deviceType: type.id,
				fields
			};
		});
	return [...templates, ...derived];
}

/** What the person typed for one field: the text, and the unit they picked. */
export interface FieldEntry {
	value: string;
	unit: string;
}

/** Trims float noise from unit conversions: 0.1 + 0.2 shows as 0.3. */
export function round(n: number) {
	return Number(n.toPrecision(10));
}

/** Converts a number typed in `unit` to the parameter's own unit. */
export function toOwnUnit(parameter: Parameter, value: number, unit: string) {
	const alt = parameter.altUnits?.find((u) => u.unit === unit);
	return alt ? round(value * alt.factor + (alt.offset ?? 0)) : value;
}

export function unitsFor(parameter: Parameter) {
	return [parameter.unit, ...(parameter.altUnits?.map((u) => u.unit) ?? [])];
}

const withUnit = (n: number, unit: string) => (unit ? `${n} ${unit}` : String(n));

/** `0 to 500 mm`, or null when the registry gives no range. */
export function rangeText(parameter: Parameter) {
	const { min, max, unit } = parameter;
	if (min === undefined && max === undefined) return null;
	if (min === undefined) return `up to ${withUnit(max!, unit)}`;
	if (max === undefined) return `at least ${withUnit(min, unit)}`;
	return `${min} to ${withUnit(max, unit)}`;
}

const inRange = (p: Parameter, n: number) =>
	(p.min === undefined || n >= p.min) && (p.max === undefined || n <= p.max);

/** One value in the parameter's own unit: the error, or null when it passes. */
function checkValue(
	parameter: Parameter,
	field: TemplateField,
	value: number | string | undefined
): string | null {
	const empty = value === undefined || (typeof value === 'string' && !value.trim());
	if (empty) return field.required ? `${parameter.label} is required.` : null;
	if ((parameter.valueType ?? 'number') === 'number') {
		if (typeof value !== 'number' || !Number.isFinite(value)) {
			return `${parameter.label} must be a number.`;
		}
		if (!inRange(parameter, value)) {
			return `${parameter.label} should be ${rangeText(parameter)}.`;
		}
		return null;
	}
	if (typeof value !== 'string') return `${parameter.label} must be text.`;
	if (parameter.options && !parameter.options.includes(value)) {
		return `Pick one of: ${parameter.options.join(', ')}.`;
	}
	return null;
}

/**
 * Checks received values against the template: required fields, numbers, plausible ranges and
 * listed answers. Values outside the template are refused. Errors are keyed by parameter ID.
 */
export function checkObservation(
	template: ObservationTemplate,
	parameters: Parameter[],
	values: Record<string, number | string>
): Record<string, string> {
	const errors: Record<string, string> = {};
	const byId = new Map(parameters.map((p) => [p.id, p]));
	for (const field of template.fields) {
		const parameter = byId.get(field.parameterId);
		if (!parameter) {
			errors[field.parameterId] = 'This value is no longer in the registry.';
			continue;
		}
		const error = checkValue(parameter, field, values[field.parameterId]);
		if (error) errors[field.parameterId] = error;
	}
	for (const id of Object.keys(values)) {
		if (!template.fields.some((f) => f.parameterId === id)) {
			errors[id] = `${template.label} does not record ${id}.`;
		}
	}
	if (!Object.keys(errors).length && !Object.keys(values).length) {
		errors.values = 'Fill in at least one value.';
	}
	return errors;
}

/**
 * Reads what was typed into values in each parameter's own unit, and checks them. When a number
 * is out of range but would fit in another of its units, the error says to switch: the usual
 * slip is measuring in °F or inches and leaving the unit as it was.
 */
export function readEntries(
	template: ObservationTemplate,
	parameters: Parameter[],
	entries: Record<string, FieldEntry>
): { values: Record<string, number | string>; errors: Record<string, string> } {
	const byId = new Map(parameters.map((p) => [p.id, p]));
	const values: Record<string, number | string> = {};
	const typed: Record<string, string> = {};
	for (const field of template.fields) {
		const parameter = byId.get(field.parameterId);
		const entry = entries[field.parameterId];
		const text = entry?.value.trim() ?? '';
		if (!parameter || !text) continue;
		if ((parameter.valueType ?? 'number') === 'number') {
			const n = Number(text.replace(',', '.'));
			if (Number.isNaN(n)) {
				typed[field.parameterId] = `${parameter.label} must be a number.`;
				continue;
			}
			values[field.parameterId] = toOwnUnit(parameter, n, entry.unit || parameter.unit);
		} else {
			values[field.parameterId] = text;
		}
	}
	const errors = { ...checkObservation(template, parameters, values), ...typed };
	for (const [id, message] of Object.entries(errors)) {
		const parameter = byId.get(id);
		const entry = entries[id];
		if (!parameter || !entry || typed[id] || typeof values[id] !== 'number') continue;
		const n = Number(entry.value.trim().replace(',', '.'));
		const picked = entry.unit || parameter.unit;
		const fits = unitsFor(parameter).find(
			(unit) => unit !== picked && inRange(parameter, toOwnUnit(parameter, n, unit))
		);
		if (fits) errors[id] = `${message} If you measured in ${fits}, switch the unit.`;
	}
	return { values, errors };
}
