import type {
	ClassroomRole,
	DeviceType,
	DeviceTypeDefinition,
	SiteSetting
} from '#lib/domain/types.ts';
import { FALLBACK_TYPE_ICON } from '#lib/icons.ts';

export const CLASSROOM_ROLE_LABELS: Record<ClassroomRole, string> = {
	institution_head: 'Institution head',
	facilitator: 'Facilitator',
	community_scientist: 'Community scientist',
	researcher: 'Researcher'
};

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

/** A device type's icon name, read from the registry like its color. */
export function deviceTypeIcon(types: DeviceTypeDefinition[], id: DeviceType) {
	return types.find((t) => t.id === id)?.icon ?? FALLBACK_TYPE_ICON;
}
