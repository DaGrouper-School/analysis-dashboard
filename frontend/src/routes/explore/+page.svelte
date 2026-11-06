<script lang="ts">
	import CoreReadingsTable from '#lib/components/CoreReadingsTable.svelte';
	import NetworkMap from '#lib/components/NetworkMap.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import SampleDataBadge from '#lib/components/SampleDataBadge.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const section = $derived(data.content.sections.find((s) => s.id === 'explore'));
</script>

<svelte:head
	><title>{section?.label ?? 'Explore data'} · {data.content.hero.eyebrow}</title></svelte:head
>

<PublicShell content={data.content} user={data.user}>
	<div class="flex flex-wrap items-center gap-3">
		<h1 class="font-display text-4xl">{section?.label ?? 'Explore data'}</h1>
		<SampleDataBadge show={data.content.sampleData} />
	</div>
	{#if section}<p class="mt-2 max-w-2xl text-muted">{section.description}</p>{/if}
	<p class="mt-1 max-w-2xl text-sm text-muted">
		Only released data is shown here. Pick a classroom to open its station.
	</p>

	<section class="mt-8" aria-label="Classroom map">
		<NetworkMap classrooms={data.classrooms} stages={data.content.stages} />
	</section>

	<section class="mt-12" aria-labelledby="core-title">
		<h2 id="core-title" class="font-display text-2xl">Core readings</h2>
		<p class="mt-1 text-sm text-muted">
			Rain counts the day of each classroom's latest released reading.
		</p>
		<div class="mt-4">
			<CoreReadingsTable classrooms={data.classrooms} units={data.units} />
		</div>
	</section>
</PublicShell>
