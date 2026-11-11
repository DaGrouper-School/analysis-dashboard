<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { navigating } from '$app/state';
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import ClassroomOverview from '#lib/components/ClassroomOverview.svelte';
	import CompletenessCard from '#lib/components/CompletenessCard.svelte';
	import DomainBrowser from '#lib/components/DomainBrowser.svelte';
	import type { GlobeMarker } from '#lib/components/Globe.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import SampleDataBadge from '#lib/components/SampleDataBadge.svelte';
	import { placeLabel } from '#lib/station-now.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const classroom = $derived(data.classroom);
	const explore = $derived(data.content.sections.find((s) => s.id === 'explore'));
	const monthName = new Intl.DateTimeFormat(undefined, {
		month: 'long',
		year: 'numeric',
		timeZone: 'UTC'
	});

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
			href: resolve('/explore/[classroom]', { classroom: p.id })
		}))
	);
	// The classroom being opened: the shared overview moves to it at once and shows a skeleton.
	const pendingId = $derived(
		(navigating.to?.route.id === '/explore/[classroom]' && navigating.to.params?.classroom) ||
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
		onpick={(id) => goto(resolve('/explore/[classroom]', { classroom: id }))}
	>
		{#snippet eyebrow(shown)}
			<div class="flex flex-wrap items-center justify-between gap-3">
				<Breadcrumbs
					items={[
						{ label: explore?.label ?? 'Explore data', href: resolve('/explore') },
						...(shown.country ? [{ label: shown.country }] : []),
						{ label: shown.name }
					]}
				/>
				<SampleDataBadge show={data.content.sampleData} />
			</div>
		{/snippet}
		{#snippet nowActions()}
			<a class="btn btn-ghost flex-1 rounded-full" href="#domains-heading">View domains</a>
		{/snippet}

		<DomainBrowser
			class="mt-12"
			domains={data.domains}
			parameters={data.parameters}
			classroomId={classroom.id}
		/>

		{#if data.completeness}
			{@const completeness = data.completeness}
			<section class="mt-12" aria-labelledby="complete-heading">
				<h2 id="complete-heading" class="font-display text-2xl">How complete is this month?</h2>
				<p class="mt-1 max-w-2xl text-sm text-muted">
					Readings received in {monthName.format(new Date(`${completeness.month}-01T00:00:00Z`))},
					as a share of what the classroom's research protocol expects. Missing days stay empty;
					they are never filled in.
				</p>
				<CompletenessCard class="mt-4" domains={completeness.domains} />
			</section>
		{/if}
	</ClassroomOverview>
</PublicShell>
