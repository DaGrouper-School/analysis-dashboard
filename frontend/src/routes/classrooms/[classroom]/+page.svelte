<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { navigating } from '$app/state';
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import ClassroomAbout from '#lib/components/ClassroomAbout.svelte';
	import ClassroomCharts from '#lib/components/ClassroomCharts.svelte';
	import ClassroomCompleteness from '#lib/components/ClassroomCompleteness.svelte';
	import ClassroomOverview from '#lib/components/ClassroomOverview.svelte';
	import DomainBrowser from '#lib/components/DomainBrowser.svelte';
	import type { GlobeMarker } from '#lib/components/Globe.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import SampleDataBadge from '#lib/components/SampleDataBadge.svelte';
	import VerticalTabs, { type TabItem } from '#lib/components/VerticalTabs.svelte';
	import { placeLabel } from '#lib/station-now.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const classroom = $derived(data.classroom);
	const directory = $derived(data.content.sections.find((s) => s.id === 'classrooms'));
	/** A classroom's page, keeping the open section tab. */
	const withTab = (id: string) =>
		resolve('/classrooms/[classroom]', { classroom: id }) + (active ? `?tab=${active}` : '');
	function pick(id: string) {
		// The path is resolved; only the query is added to it.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(withTab(id));
	}

	// The same section tabs as the dashboard, with what a visitor may see.
	type Section = 'domains' | 'complete' | 'about';
	let active = $state<Section>();
	const sections = $derived<TabItem<Section>[]>([
		{ id: 'domains', label: 'Domains', icon: 'layers', count: data.domains.length },
		{ id: 'complete', label: 'Completeness', icon: 'check' },
		{ id: 'about', label: 'About', icon: 'info' }
	]);
	let sectionsEl = $state<HTMLElement>();
	function openSection(id: Section) {
		active = id;
		const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
		sectionsEl?.scrollIntoView({ behavior: reduce ? 'auto' : 'smooth', block: 'start' });
	}

	// Every public classroom as a tab and on the globe, coloured by its stage.
	const stageColor = $derived(new Map(data.content.stages.map((s) => [s.id, s.color])));
	const markers = $derived<GlobeMarker[]>(
		data.places.map((p) => ({
			id: p.id,
			lat: p.location!.lat,
			lng: p.location!.lng,
			color: stageColor.get(p.stageId) ?? '#8e98a3',
			label: p.name
		}))
	);
	const tabs = $derived(
		data.places.map((p) => ({
			id: p.id,
			label: placeLabel(p.placeName, p.country) || p.name,
			title: p.name,
			href: withTab(p.id)
		}))
	);
	// The classroom being opened: the shared overview moves to it at once and shows a skeleton.
	const pendingId = $derived(
		(navigating.to?.route.id === '/classrooms/[classroom]' && navigating.to.params?.classroom) ||
			classroom.id
	);
</script>

<svelte:head><title>{classroom.name} · {data.content.hero.eyebrow}</title></svelte:head>

<PublicShell content={data.content} user={data.user}>
	<ClassroomOverview
		station={data}
		{tabs}
		tabsLabel="Locations"
		{pendingId}
		{markers}
		onpick={pick}
	>
		{#snippet eyebrow(shown)}
			<div class="flex flex-wrap items-center justify-between gap-3">
				<Breadcrumbs
					items={[
						{ label: directory?.label ?? 'Classrooms', href: resolve('/classrooms') },
						...(shown.country ? [{ label: shown.country }] : []),
						{ label: shown.name }
					]}
				/>
				<SampleDataBadge show={data.content.sampleData} />
			</div>
		{/snippet}
		{#snippet nowActions()}
			<button
				type="button"
				class="btn btn-ghost flex-1 rounded-full"
				onclick={() => openSection('domains')}>View domains</button
			>
		{/snippet}

		{#snippet institutionActions()}
			<button
				type="button"
				class="btn btn-ghost flex-1 rounded-full"
				onclick={() => openSection('about')}>About {classroom.name}</button
			>
		{/snippet}

		<ClassroomCharts class="mt-12" station={data} />

		<section bind:this={sectionsEl} class="mt-14 scroll-mt-28" aria-label="Classroom sections">
			<VerticalTabs tabs={sections} bind:active label="Classroom sections">
				{#if active === 'domains'}
					<DomainBrowser
						domains={data.domains}
						parameters={data.parameters}
						classroomId={classroom.id}
					/>
				{:else if active === 'complete'}
					<ClassroomCompleteness completeness={data.completeness} />
				{:else if active === 'about'}
					<ClassroomAbout about={data.about} join={!data.user} />
				{/if}
			</VerticalTabs>
		</section>
	</ClassroomOverview>
</PublicShell>
