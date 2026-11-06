<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import NetworkMap from '#lib/components/NetworkMap.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import StageBadge from '#lib/components/StageBadge.svelte';
	import {
		filterClassrooms,
		filterOptions,
		filterQuery,
		groupClassrooms,
		NO_FILTER,
		type DirectoryFilter
	} from '#lib/directory.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const content = $derived(data.content);
	const section = $derived(content.sections.find((s) => s.id === 'classrooms'));
	const stages = $derived(new Map(content.stages.map((s) => [s.id, s])));
	const options = $derived(filterOptions(data.classrooms, content));
	const shown = $derived(filterClassrooms(data.classrooms, data.filter));
	const groups = $derived(groupClassrooms(shown, content.regions));
	const filtered = $derived(Object.values(data.filter).some(Boolean));

	/** Changes one filter, kept in the URL so the view can be shared. */
	function set(change: Partial<DirectoryFilter>) {
		// Only the query is added to a resolved path.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		goto(resolve('/global-classrooms') + filterQuery({ ...data.filter, ...change }), {
			replace: true,
			reset: false
		});
	}
</script>

<svelte:head
	><title>{section?.label ?? 'Global Classrooms'} · {content.hero.eyebrow}</title></svelte:head
>

{#snippet picker(
	id: keyof DirectoryFilter,
	label: string,
	any: string,
	items: { value: string; label: string }[]
)}
	<div>
		<label class="label" for="filter-{id}">{label}</label>
		<select
			id="filter-{id}"
			class="input pr-9"
			value={data.filter[id]}
			onchange={(e) => set({ [id]: e.currentTarget.value })}
		>
			<option value="">{any}</option>
			{#each items as item (item.value)}
				<option value={item.value}>{item.label}</option>
			{/each}
		</select>
	</div>
{/snippet}

<PublicShell {content} user={data.user}>
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="font-display text-4xl">{section?.label ?? 'Global Classrooms'}</h1>
			{#if section}<p class="mt-2 max-w-2xl text-muted">{section.description}</p>{/if}
		</div>
		<a class="btn btn-primary" href={resolve('/host')}>{content.hosting.title}</a>
	</div>

	<form
		class="mt-8 grid gap-4 sm:grid-cols-[repeat(3,minmax(0,1fr))_auto] sm:items-end"
		aria-label="Filter classrooms"
		onsubmit={(e) => e.preventDefault()}
	>
		{@render picker(
			'region',
			'Region',
			'All regions',
			options.regions.map((r) => ({ value: r.id, label: r.label }))
		)}
		{@render picker(
			'country',
			'Country',
			'All countries',
			options.countries.map((c) => ({ value: c, label: c }))
		)}
		{@render picker(
			'stage',
			'Stage',
			'All stages',
			options.stages.map((s) => ({ value: s.id, label: s.label }))
		)}
		<button type="button" class="btn btn-ghost" disabled={!filtered} onclick={() => set(NO_FILTER)}
			>Clear</button
		>
	</form>
	<p class="mt-3 text-sm text-muted" aria-live="polite">
		{shown.length} of {data.classrooms.length} Global Classroom{data.classrooms.length === 1
			? ''
			: 's'}
	</p>

	<section class="mt-6" aria-label="Map of the classrooms shown">
		<NetworkMap classrooms={shown} stages={content.stages} />
	</section>

	{#each groups as group (group.region?.id ?? 'none')}
		<section class="mt-12" aria-labelledby="region-{group.region?.id ?? 'none'}">
			<h2 id="region-{group.region?.id ?? 'none'}" class="font-display text-2xl">
				{group.region?.label ?? 'Region not set yet'}
			</h2>
			{#each group.countries as { country, classrooms } (country ?? 'none')}
				<h3 class="mt-5 text-sm font-medium text-muted">{country ?? 'Country not set yet'}</h3>
				<ul class="mt-2 grid gap-4 md:grid-cols-2">
					{#each classrooms as classroom (classroom.id)}
						<li class="card flex flex-col gap-3">
							<div class="flex items-start justify-between gap-3">
								<div class="min-w-0">
									<h4 class="text-lg font-semibold text-ink">
										<a
											class="hover:underline"
											href={resolve('/global-classrooms/[classroom]', {
												classroom: classroom.id
											})}>{classroom.name}</a
										>
									</h4>
									<p class="text-sm text-muted">
										{[classroom.placeName, classroom.country].filter(Boolean).join(', ') ||
											classroom.institutionName}
									</p>
								</div>
								<StageBadge stage={stages.get(classroom.stageId)} stageId={classroom.stageId} />
							</div>
							{#if classroom.themes.length}
								<div>
									<p class="text-xs text-muted">Researching</p>
									<ul class="mt-1 list-disc space-y-1 pl-5 text-sm">
										{#each classroom.themes as theme (theme)}<li>{theme}</li>{/each}
									</ul>
								</div>
							{/if}
							<a
								class="link mt-auto self-start text-sm"
								href={resolve('/explore/[classroom]', { classroom: classroom.id })}
								>See {classroom.name}'s data</a
							>
						</li>
					{/each}
				</ul>
			{/each}
		</section>
	{:else}
		<p class="mt-10 text-muted">No classroom matches these filters.</p>
	{/each}
</PublicShell>
