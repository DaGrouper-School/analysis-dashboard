import type { TabItem } from '#lib/components/VerticalTabs.svelte';

/** The classroom's sections on the Classrooms page (AP-45). */
export type Section = 'devices' | 'labs' | 'records' | 'complete' | 'about' | 'people' | 'settings';

interface SectionCounts {
	devices: number;
	labs: number;
	records: number;
	members: number;
}

/**
 * The section tabs, in order, for the classroom's members (a visitor never opens the page):
 * Measurements first; Labs only for the institution head, People for whoever manages members,
 * invites or consent, and Settings for whoever edits. Domains have their own page, and alerts
 * and messages are in the top bar.
 */
export function sectionTabs(
	counts: SectionCounts,
	who: { head: boolean; managesPeople: boolean; managesSettings: boolean }
): TabItem<Section>[] {
	return [
		{ id: 'devices', label: 'Measurements', icon: 'station', count: counts.devices },
		...(who.head
			? [{ id: 'labs' as const, label: 'Labs', icon: 'flask', count: counts.labs }]
			: []),
		{ id: 'records', label: 'Manual records', icon: 'clipboard', count: counts.records },
		{ id: 'complete', label: 'Completeness', icon: 'check' },
		{ id: 'about', label: 'About', icon: 'info' },
		...(who.managesPeople
			? [{ id: 'people' as const, label: 'People', icon: 'users', count: counts.members }]
			: []),
		...(who.managesSettings
			? [{ id: 'settings' as const, label: 'Settings', icon: 'sliders' }]
			: [])
	];
}
