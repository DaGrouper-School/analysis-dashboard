import type {
	DeviceTypeDefinition,
	DeviceTypeInput,
	DomainDefinition,
	ExpectedValueInput,
	Parameter
} from '#lib/domain/types.ts';
import type { SelectOption } from '#lib/components/Select.svelte';
import { FALLBACK_TYPE_ICON } from '#lib/utils/icons.ts';
import { DEVICE_TYPE_PALETTE } from '#lib/utils/labels.ts';

/** The domain pickers' last choice: make a new domain on the spot and use it. */
export const NEW_DOMAIN = '__new__';

/** Where a value can go: no domain, one of the registry's, or a new one made on the spot. */
export function domainOptions(domains: DomainDefinition[]): SelectOption[] {
	return [
		{ value: '', label: 'Not in a domain' },
		...domains.map((d) => ({ value: d.id, label: d.label, hint: d.description, icon: d.icon })),
		{ value: NEW_DOMAIN, label: 'New domain…', icon: 'plus' }
	];
}

export const VALUE_KINDS: SelectOption<ExpectedValueInput['valueType']>[] = [
	{ value: 'number', label: 'Number' },
	{ value: 'text', label: 'Text' },
	{ value: 'time', label: 'Time (Unix seconds)' }
];

export const TYPE_STEPS = [
	{ label: 'Basics', icon: 'tag' },
	{ label: 'Expected values', icon: 'readings' },
	{ label: 'Review', icon: 'review' }
];

/** One expected value in the form. `key` keeps a row's fields while rows are added or removed. */
export type ValueRow = ExpectedValueInput & { key: number };

/** The measurement type form: the basics, then one row per expected value. */
export interface TypeForm {
	label: string;
	description: string;
	color: string;
	icon: string;
	manual: boolean;
	rows: ValueRow[];
}

let nextKey = 0;

export const blankRow = (): ValueRow => ({
	key: nextKey++,
	id: '',
	label: '',
	unit: '',
	valueType: 'number',
	domain: null,
	tile: false,
	card: false
});

export const blankType = (): TypeForm => ({
	label: '',
	description: '',
	color: DEVICE_TYPE_PALETTE[5],
	icon: FALLBACK_TYPE_ICON,
	manual: false,
	rows: [blankRow()]
});

/** The form for editing `type`, its rows filled from the registry's values. */
export function typeForm(
	type: DeviceTypeDefinition,
	parameters: ReadonlyMap<string, Parameter>
): TypeForm {
	const rows = type.parameterIds.map((id): ValueRow => {
		const p = parameters.get(id);
		return {
			key: nextKey++,
			id,
			label: p?.label ?? id,
			unit: p?.unit ?? '',
			valueType: p?.valueType ?? 'number',
			domain: p?.domain ?? null,
			tile: type.tileParameterIds.includes(id),
			card: type.cardParameterIds.includes(id)
		};
	});
	return {
		label: type.label,
		description: type.description,
		color: type.color,
		icon: type.icon,
		manual: type.manual === true,
		rows: rows.length ? rows : [blankRow()]
	};
}

/** Typing an ID the registry has fills in that value: the type will share it. */
export function fillRow(row: ValueRow, id: string, parameters: ReadonlyMap<string, Parameter>) {
	row.id = id;
	const existing = parameters.get(id.trim());
	if (existing) {
		row.label = existing.label;
		row.unit = existing.unit;
		row.valueType = existing.valueType ?? 'number';
		row.domain = existing.domain;
	}
}

/** A row with an ID or a label: blank rows are left out of the input. */
export const isFilled = (row: ValueRow) => Boolean(row.id.trim() || row.label.trim());

/** The form as the service's input, without blank rows. */
export function typeInput(form: TypeForm): DeviceTypeInput {
	return {
		label: form.label,
		description: form.description,
		color: form.color,
		icon: form.icon,
		manual: form.manual,
		values: form.rows.filter(isFilled).map((r) => ({
			id: r.id,
			label: r.label,
			unit: r.unit,
			valueType: r.valueType,
			domain: r.domain,
			tile: r.tile,
			card: r.card
		}))
	};
}

/** Which step holds a field's error: values on step two, the rest on step one. */
export const typeStepOf = (field: string) => (field.startsWith('values') ? 1 : 0);

/** The first step with one of `errors`, or null when there are none. */
export function firstStepWithError(errors: Record<string, string>): number | null {
	const fields = Object.keys(errors);
	return fields.length ? Math.min(...fields.map(typeStepOf)) : null;
}
