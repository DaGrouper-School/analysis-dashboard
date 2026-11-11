import type { TabItem } from '#lib/components/VerticalTabs.svelte';

/** The classroom's sections on the details page (AP-45). */
export type Section =
	| 'alerts'
	| 'domains'
	| 'devices'
	| 'labs'
	| 'records'
	| 'complete'
	| 'about'
	| 'people'
	| 'settings';

interface SectionCounts {
	domains: number;
	/** Alerts and members waiting for consent: what needs the user. */
	alerts: number;
	devices: number;
	labs: number;
	records: number;
	members: number;
}

/**
 * The section tabs, in order. A visitor sees the public ones. Members see Domains first; Labs
 * only for the institution head, People for whoever manages members, invites or consent, and
 * Settings for whoever edits.
 */
export function sectionTabs(
	counts: SectionCounts,
	who: { visiting: boolean; head: boolean; managesPeople: boolean; managesSettings: boolean }
): TabItem<Section>[] {
	const domains: TabItem<Section> = {
		id: 'domains',
		label: 'Domains',
		icon: 'layers',
		count: counts.domains
	};
	const common: TabItem<Section>[] = [
		{ id: 'complete', label: 'Completeness', icon: 'check' },
		{ id: 'about', label: 'About', icon: 'info' }
	];
	if (who.visiting) return [domains, ...common];
	return [
		domains,
		{
			id: 'alerts',
			label: 'Alerts',
			icon: 'alert',
			...(counts.alerts
				? { badge: { text: String(counts.alerts), title: `${counts.alerts} need you` } }
				: { count: 0 })
		},
		{ id: 'devices', label: 'Measurements', icon: 'station', count: counts.devices },
		...(who.head
			? [{ id: 'labs' as const, label: 'Labs', icon: 'flask', count: counts.labs }]
			: []),
		{ id: 'records', label: 'Manual records', icon: 'clipboard', count: counts.records },
		...common,
		...(who.managesPeople
			? [{ id: 'people' as const, label: 'People', icon: 'users', count: counts.members }]
			: []),
		...(who.managesSettings
			? [{ id: 'settings' as const, label: 'Settings', icon: 'sliders' }]
			: [])
	];
}
