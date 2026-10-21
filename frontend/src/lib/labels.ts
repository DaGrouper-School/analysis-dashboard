import type {
	ClassroomRole,
	DeviceType,
	DeviceTypeDefinition,
	SiteSetting
} from '#lib/domain/types.ts';

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

/** A device type's marker and chip color, from the registry's device types. */
export function deviceTypeColor(types: DeviceTypeDefinition[], id: DeviceType) {
	return types.find((t) => t.id === id)?.color ?? '#9aa5ae';
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
