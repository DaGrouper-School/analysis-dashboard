/**
 * The app's line icons: 24 × 24, drawn with `currentColor` so they follow the theme. Device types
 * and domains store one of the registry icon names; platform admins pick it in the Registry.
 */

type Shape =
	| { d: string }
	| { cx: number; cy: number; r: number }
	| { x: number; y: number; width: number; height: number; rx?: number };

export type IconShape = Shape;

/** Icons an admin can pick for a device type or a domain, in picker order. */
export const REGISTRY_ICONS = {
	station: [
		{ d: 'M12 8v14M8.5 22h7M12 8 6.5 5M12 8l5.5-3M12 8V3' },
		{ cx: 5.5, cy: 4.5, r: 1.5 },
		{ cx: 18.5, cy: 4.5, r: 1.5 },
		{ d: 'M12 13h3.5v3H12' }
	],
	thermometer: [{ d: 'M14 14.8V4.5a2 2 0 1 0-4 0v10.3a4 4 0 1 0 4 0ZM12 9.5v8' }],
	droplet: [{ d: 'M12 3s6.5 6.6 6.5 11.2a6.5 6.5 0 0 1-13 0C5.5 9.6 12 3 12 3Z' }],
	rain: [
		{ d: 'M7 15.5a4.5 4.5 0 0 1-.4-9A6 6 0 0 1 18 7.6a3.9 3.9 0 0 1-.5 7.9' },
		{ d: 'M8 18.5 7 21M12.5 17 11 21.5M17 18.5 16 21' }
	],
	gauge: [{ d: 'M8 3h8M9 3v15.5a3 3 0 0 0 6 0V3M9 8h3M9 12h4M9 16h3' }],
	wind: [{ d: 'M3 8h10a2.5 2.5 0 1 0-2.5-2.5M3 12h15a2.5 2.5 0 1 1-2.5 2.5M3 16h7a2 2 0 1 1-2 2' }],
	sun: [
		{ cx: 12, cy: 12, r: 4 },
		{
			d: 'M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4'
		}
	],
	waves: [
		{
			d: 'M2 7.5c1.7 0 2.3-1.5 4-1.5s2.3 1.5 4 1.5 2.3-1.5 4-1.5 2.3 1.5 4 1.5 2.3-1.5 4-1.5M2 12.5c1.7 0 2.3-1.5 4-1.5s2.3 1.5 4 1.5 2.3-1.5 4-1.5 2.3 1.5 4 1.5 2.3-1.5 4-1.5M2 17.5c1.7 0 2.3-1.5 4-1.5s2.3 1.5 4 1.5 2.3-1.5 4-1.5 2.3 1.5 4 1.5 2.3-1.5 4-1.5'
		}
	],
	bolt: [{ d: 'M13.5 2 4.5 13.5h6.5L10 22l9-11.5h-6.5L13.5 2Z' }],
	tree: [{ d: 'M12 22v-5M12 2.5 6.5 10h3L5 16.5h14L14.5 10h3L12 2.5Z' }],
	sprout: [
		{
			d: 'M12 21v-9M12 12C12 7.6 9.2 5 4 5c0 4.4 2.8 7 8 7ZM12 14.5c0-3.6 2.4-6 7-6 0 3.6-2.4 6-7 6Z'
		},
		{ d: 'M7 21h10' }
	],
	soil: [
		{ d: 'M2.5 9h19' },
		{ d: 'M12 2.5v13.5M9.5 16h5L12 21.5 9.5 16Z' },
		{ d: 'M4 13h3M17 13h3M5.5 17h2M16.5 17h2' }
	],
	water: [
		{ d: 'M12 2.5v8M10 10.5h4V15a2 2 0 0 1-4 0v-4.5Z' },
		{ d: 'M2 19c1.7 0 2.3-1.3 4-1.3s2.3 1.3 4 1.3M14 19c1.7 0 2.3-1.3 4-1.3s2.3 1.3 4 1.3' }
	],
	particles: [
		{ cx: 6.5, cy: 7, r: 2 },
		{ cx: 15, cy: 5, r: 1.5 },
		{ cx: 17.5, cy: 12.5, r: 2.5 },
		{ cx: 9.5, cy: 14, r: 1.5 },
		{ cx: 14, cy: 19.5, r: 1.5 },
		{ cx: 5, cy: 18.5, r: 1 }
	],
	cloud: [{ d: 'M7 19a4.5 4.5 0 0 1-.6-9A6.5 6.5 0 0 1 19 10.6 4.2 4.2 0 0 1 17.5 19H7Z' }],
	haze: [{ d: 'M3 9h9.5a2.5 2.5 0 1 0-2.5-2.5M3 13h15a2.5 2.5 0 1 1-2.5 2.5M5 17h6M14 9h5' }],
	snow: [
		{
			d: 'M12 2.5v19M3.8 7.3l16.4 9.4M20.2 7.3 3.8 16.7M9.5 3.5 12 6l2.5-2.5M9.5 20.5 12 18l2.5 2.5'
		}
	],
	mountain: [{ d: 'M2.5 20 9 8.5l4 6.5 2.5-3.5L21.5 20h-19Z' }],
	compass: [{ cx: 12, cy: 12, r: 9 }, { d: 'm15.5 8.5-2 5-5 2 2-5 5-2Z' }],
	radar: [
		{ cx: 12, cy: 12, r: 9 },
		{ cx: 12, cy: 12, r: 5 },
		{ d: 'M12 12l6.4-6.4' },
		{ cx: 12, cy: 12, r: 1 }
	],
	antenna: [
		{ cx: 12, cy: 9, r: 1.5 },
		{
			d: 'M12 10.5V22M8.5 22h7M8.2 5.2a5.4 5.4 0 0 0 0 7.6M15.8 5.2a5.4 5.4 0 0 1 0 7.6M5.4 2.4a9.4 9.4 0 0 0 0 13.2M18.6 2.4a9.4 9.4 0 0 1 0 13.2'
		}
	],
	satellite: [
		{ d: 'm9.5 9.5 3-3 5 5-3 3-5-5Z' },
		{ d: 'm2.5 8 3.5-3.5 3 3L5.5 11l-3-3ZM13 18.5l3.5-3.5 3 3-3.5 3.5-3-3Z' },
		{ d: 'M7.5 6 10 8.5M14 15.5l2.5 2.5M3 21a4 4 0 0 1 4-4' }
	],
	flask: [
		{
			d: 'M9 3h6M10 3v6.2l-5.3 9.3A1.7 1.7 0 0 0 6.2 21h11.6a1.7 1.7 0 0 0 1.5-2.5L14 9.2V3M7.4 15h9.2'
		}
	],
	pin: [
		{ d: 'M12 21.5s7-6.2 7-11.5a7 7 0 0 0-14 0c0 5.3 7 11.5 7 11.5Z' },
		{ cx: 12, cy: 10, r: 2.5 }
	],
	sensor: [
		{ x: 6, y: 6, width: 12, height: 12, rx: 2 },
		{ x: 9.5, y: 9.5, width: 5, height: 5, rx: 0.5 },
		{
			d: 'M9.5 2.5V6M14.5 2.5V6M9.5 18v3.5M14.5 18v3.5M2.5 9.5H6M2.5 14.5H6M18 9.5h3.5M18 14.5h3.5'
		}
	],
	tag: [{ d: 'M3 12.2V4a1 1 0 0 1 1-1h8.2l8.8 8.8-9.2 9.2L3 12.2Z' }, { cx: 7.5, cy: 7.5, r: 1.5 }]
} satisfies Record<string, Shape[]>;

/** Icons for actions and pairing steps. Not offered in the registry. */
export const UI_ICONS = {
	sync: [{ d: 'M20 11a8 8 0 0 0-14.3-4.9L4 8M4 4v4h4M4 13a8 8 0 0 0 14.3 4.9L20 16M20 20v-4h-4' }],
	readings: [{ d: 'M4 5h16M4 10h16M4 15h10M4 20h7' }, { cx: 18.5, cy: 18.5, r: 2.5 }],
	plus: [{ d: 'M12 5v14M5 12h14' }],
	check: [{ d: 'm5 12.5 4.5 4.5L19 7.5' }],
	close: [{ d: 'M6 6l12 12M18 6 6 18' }],
	link: [
		{
			d: 'M10 14a4 4 0 0 0 5.7 0l3.5-3.5a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3.5 3.5a4 4 0 0 0 5.7 5.7l1-1'
		}
	],
	upload: [{ d: 'M12 16V4M7 9l5-5 5 5M4 15v4a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-4' }],
	alert: [{ d: 'M12 3 2 20.5h20L12 3ZM12 10v4.5M12 17.5v.5' }],
	info: [{ cx: 12, cy: 12, r: 9 }, { d: 'M12 11v5.5M12 7.5v.5' }],
	arrow: [{ d: 'M5 12h14M13 6l6 6-6 6' }]
} satisfies Record<string, Shape[]>;

export type RegistryIconName = keyof typeof REGISTRY_ICONS;
export type IconName = RegistryIconName | keyof typeof UI_ICONS;

export const REGISTRY_ICON_NAMES = Object.keys(REGISTRY_ICONS) as RegistryIconName[];

/** Shown when a registry entry has no icon, or one this build doesn't know. */
export const FALLBACK_TYPE_ICON: RegistryIconName = 'sensor';
export const FALLBACK_DOMAIN_ICON: RegistryIconName = 'tag';

export function isRegistryIcon(name: string): name is RegistryIconName {
	return Object.hasOwn(REGISTRY_ICONS, name);
}

export function iconShapes(name: string, fallback: RegistryIconName = 'sensor'): Shape[] {
	if (isRegistryIcon(name)) return REGISTRY_ICONS[name];
	if (Object.hasOwn(UI_ICONS, name)) return UI_ICONS[name as keyof typeof UI_ICONS];
	return REGISTRY_ICONS[fallback];
}

/** `particles` reads as "Particles" in the picker. */
export function iconLabel(name: string) {
	return name.replace(/^./, (c) => c.toUpperCase());
}
