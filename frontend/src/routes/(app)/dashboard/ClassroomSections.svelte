<script lang="ts">
	import VerticalTabs, { type TabItem } from '#lib/components/VerticalTabs.svelte';
	import type { Parameter, PublicStation } from '#lib/domain/types.ts';
	import AlertsList from '#lib/features/alerts/AlertsList.svelte';
	import ClassroomAbout from '#lib/features/classroom/ClassroomAbout.svelte';
	import ClassroomCompleteness from '#lib/features/classroom/ClassroomCompleteness.svelte';
	import ClassroomPeople from '#lib/features/classroom/ClassroomPeople.svelte';
	import ClassroomSettings from '#lib/features/classroom/ClassroomSettings.svelte';
	import ClassroomLabs from '#lib/features/devices/ClassroomLabs.svelte';
	import DomainBrowser from '#lib/features/measurements/DomainBrowser.svelte';
	import type { domainsOf } from '#lib/features/measurements/domains.ts';
	import MeasurementsPanel from './MeasurementsPanel.svelte';
	import PanelHead from './PanelHead.svelte';
	import RecordsPanel from './RecordsPanel.svelte';
	import type { Section } from './sections.ts';
	import type { PageData } from './$types';

	/** The classroom's sections as tabs on the details page (AP-45), one panel per tab. */
	let {
		data,
		station,
		sections,
		active = $bindable(),
		domains,
		parameters,
		alertCount,
		canAddDevice,
		canAddLab,
		onsync,
		onconsent
	}: {
		data: PageData;
		station: PublicStation;
		sections: TabItem<Section>[];
		active: Section | undefined;
		domains: ReturnType<typeof domainsOf>;
		parameters: Map<string, Parameter>;
		alertCount: number;
		canAddDevice: boolean;
		canAddLab: boolean;
		onsync: (deviceId: string) => void;
		onconsent: () => void;
	} = $props();
</script>

<section class="mt-8" aria-label="Classroom sections">
	<VerticalTabs tabs={sections} bind:active label="Classroom sections">
		{#if active === 'alerts'}
			<PanelHead
				title="Alerts"
				sub={alertCount
					? `What needs you in ${station.classroom.name}, from each device's latest reading.`
					: `Nothing needs you in ${station.classroom.name} right now.`}
			/>
			<AlertsList
				class="mt-5"
				alerts={data.alerts}
				waiting={data.waitingForConsent}
				{parameters}
				classroomId={station.classroom.id}
				{onconsent}
			/>
		{:else if active === 'domains'}
			<DomainBrowser {domains} parameters={station.parameters} classroomId={station.classroom.id} />
		{:else if active === 'devices'}
			<MeasurementsPanel
				{data}
				classroomId={station.classroom.id}
				classroomName={station.classroom.name}
				{parameters}
				{canAddDevice}
				{onsync}
			/>
		{:else if active === 'labs'}
			<PanelHead
				title="Labs"
				sub={`${station.classroom.name}'s labs, with their devices and site profile.`}
			/>
			<div class="mt-5">
				<ClassroomLabs
					classroomId={station.classroom.id}
					labs={data.labs}
					devices={data.devices}
					canAdd={canAddLab}
				/>
			</div>
		{:else if active === 'records'}
			<RecordsPanel records={data.records} canRecord={data.canRecord} {parameters} />
		{:else if active === 'complete'}
			<ClassroomCompleteness completeness={station.completeness} />
		{:else if active === 'about' && data.about}
			<ClassroomAbout about={data.about} />
		{:else if active === 'people' && data.detail}
			<PanelHead
				title="People"
				sub={`Who is in ${station.classroom.name}, who is invited, and guardian consent for members under 18.`}
			/>
			<div class="mt-5">
				<ClassroomPeople detail={data.detail} consent={data.consent} userId={data.user.id} />
			</div>
		{:else if active === 'settings' && data.detail}
			<PanelHead title="Settings" sub={`${station.classroom.name}'s name, institution and code.`} />
			<div class="mt-5">
				<ClassroomSettings detail={data.detail} institutionInfo={station.institution} />
			</div>
		{/if}
	</VerticalTabs>
</section>
