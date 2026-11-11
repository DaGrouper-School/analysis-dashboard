import type {
	ClassroomRole,
	ClassroomStage,
	DeviceType,
	DeviceTypeDefinition,
	SiteSetting
} from '#lib/domain/types.ts';
import type { SelectOption } from '#lib/components/Select.svelte';
import { FALLBACK_TYPE_ICON } from '#lib/utils/icons.ts';

export const CLASSROOM_ROLE_LABELS: Record<ClassroomRole, string> = {
	institution_head: 'Institution head',
	facilitator: 'Facilitator',
	community_scientist: 'Community scientist',
	researcher: 'Researcher'
};

/** "a researcher", "an institution head". */
export const withArticle = (noun: string) => `${/^[aeiou]/i.test(noun) ? 'an' : 'a'} ${noun}`;

export const SITE_SETTING_LABELS: Record<SiteSetting, string> = {
	coastal: 'Coastal',
	urban: 'Urban',
	rural: 'Rural or agricultural',
	highland: 'Highland',
	forest: 'Forest',
	other: 'Other'
};

/** A device type's name, from the registry's device types. */
export function deviceTypeLabel(types: DeviceTypeDefinition[], id: DeviceType) {
	return (
		types.find((t) => t.id === id)?.label ??
		id.replaceAll('_', ' ').replace(/^./, (c) => c.toUpperCase())
	);
}

/** Colors offered for a new device type, the built-in types' own first. */
export const DEVICE_TYPE_PALETTE = [
	'#3cc04b',
	'#00bdff',
	'#f6921e',
	'#2f7ff0',
	'#e8457a',
	'#a77bf3',
	'#d4b72c',
	'#9aa5ae'
];

/** Color a device whose type is no longer in the registry falls back to: the palette's gray. */
const UNKNOWN_TYPE_COLOR = DEVICE_TYPE_PALETTE.at(-1)!;

/**
 * A device type's marker, chip and card color, read from the registry's device types, so a
 * color the admin picks shows everywhere. Never hard-code a type's color in a page.
 */
export function deviceTypeColor(types: DeviceTypeDefinition[], id: DeviceType) {
	return types.find((t) => t.id === id)?.color ?? UNKNOWN_TYPE_COLOR;
}

/** A classroom stage's marker and badge color, read from the site's stages like a type's. */
export function stageColor(stages: ClassroomStage[], id: string) {
	return stages.find((s) => s.id === id)?.color ?? UNKNOWN_TYPE_COLOR;
}

/** A device type's registry entry, for its tile and card values and the rest. */
export function deviceTypeOf(types: DeviceTypeDefinition[], id: DeviceType) {
	return types.find((t) => t.id === id);
}

/** A device type's icon name, read from the registry like its color. */
export function deviceTypeIcon(types: DeviceTypeDefinition[], id: DeviceType) {
	return types.find((t) => t.id === id)?.icon ?? FALLBACK_TYPE_ICON;
}

/** Whether a type is entered by hand, with no sensor to pair or sync. */
export function isManualType(types: DeviceTypeDefinition[], id: DeviceType) {
	return types.find((t) => t.id === id)?.manual === true;
}

/**
 * Devices as dropdown choices, each with its type's registry color and icon, and the type's
 * name (or the device's identifier) under it.
 */
export function deviceOptions(
	devices: readonly { id: string; name: string; type: DeviceType; identifier?: string }[],
	types: DeviceTypeDefinition[],
	hint: 'type' | 'identifier' = 'type'
): SelectOption[] {
	return devices.map((d) => ({
		value: d.id,
		label: d.name,
		hint: hint === 'type' ? deviceTypeLabel(types, d.type) : d.identifier,
		color: deviceTypeColor(types, d.type),
		icon: deviceTypeIcon(types, d.type)
	}));
}
