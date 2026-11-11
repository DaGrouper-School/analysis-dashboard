<script lang="ts">
	import { resolve } from '$app/paths';
	import CoreReadingsTable from '#lib/components/CoreReadingsTable.svelte';
	import Globe from '#lib/components/Globe.svelte';
	import NetworkMap from '#lib/components/NetworkMap.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import SampleDataBadge from '#lib/components/SampleDataBadge.svelte';
	import { sectionHref } from '#lib/site.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const content = $derived(data.content);
	const directory = $derived(content.sections.find((s) => s.id === 'classrooms'));
</script>

<svelte:head>
	<title>{content.hero.eyebrow}</title>
	<meta name="description" content={content.hero.body} />
</svelte:head>

<PublicShell {content} user={data.user}>
	<section class="grid items-center gap-10 py-6 lg:grid-cols-[1.1fr_1fr]">
		<div>
			<p class="text-sm font-medium tracking-wide text-lime uppercase">{content.hero.eyebrow}</p>
			<h1 class="mt-3 font-display text-4xl leading-tight sm:text-5xl">{content.hero.title}</h1>
			<p class="mt-5 max-w-xl text-lg">{content.hero.body}</p>
			<div class="mt-8 flex flex-wrap gap-3">
				<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
				<a href={directory ? sectionHref(directory) : '#network'} class="btn btn-primary"
					>Explore the data</a
				>
				<a href={resolve('/host')} class="btn btn-ghost">{content.hosting.title}</a>
			</div>
		</div>
		<div class="hidden h-96 lg:block" aria-hidden="true">
			<Globe markers={[]} interactive={false} />
		</div>
	</section>

	<section id="network" class="mt-16 scroll-mt-24" aria-labelledby="network-title">
		<h2 id="network-title" class="font-display text-3xl">The network</h2>
		<p class="mt-2 text-muted">
			{data.classrooms.length} Global Classroom{data.classrooms.length === 1 ? '' : 's'}, each at
			its own stage.
		</p>
		<div class="mt-6">
			<NetworkMap classrooms={data.classrooms} stages={content.stages} />
		</div>
	</section>

	<section class="mt-16" aria-labelledby="core-title">
		<h2 id="core-title" class="flex flex-wrap items-center gap-3 font-display text-3xl">
			Core readings <SampleDataBadge show={content.sampleData} />
		</h2>
		<p class="mt-2 max-w-2xl text-muted">
			The same few values from every classroom, from released data only. Rain counts the day of each
			classroom's latest released reading.
		</p>
		<div class="mt-6">
			<CoreReadingsTable classrooms={data.classrooms} units={data.units} />
		</div>
	</section>

	<section class="mt-16" aria-labelledby="doors-title">
		<h2 id="doors-title" class="font-display text-3xl">Find your way in</h2>
		<ul class="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			{#each content.sections as section (section.id)}
				<li>
					<!-- eslint-disable svelte/no-navigation-without-resolve -->
					<a
						href={sectionHref(section)}
						class="card flex h-full flex-col gap-2 transition hover:border-line-strong"
					>
						<span class="flex items-start justify-between gap-3">
							<span class="text-lg font-semibold text-ink">{section.label}</span>
							{#if !section.href}<span class="chip shrink-0 text-xs">Coming soon</span>{/if}
						</span>
						<span class="text-sm text-muted">{section.description}</span>
					</a>
					<!-- eslint-enable svelte/no-navigation-without-resolve -->
				</li>
			{/each}
		</ul>
	</section>
</PublicShell>
