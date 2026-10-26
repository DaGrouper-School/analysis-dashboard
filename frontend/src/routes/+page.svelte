<script lang="ts">
	import Globe, { type GlobeMarker } from '#lib/components/Globe.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import type { ReviewState } from '#lib/domain/types.ts';
	import { formatValue } from '#lib/key-values.ts';
	import { sectionHref } from '#lib/site.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const content = $derived(data.content);
	const stages = $derived(new Map(content.stages.map((s) => [s.id, s])));
	const host = $derived(content.sections.find((s) => s.id === 'classrooms'));
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const dayFormat = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeZone: 'UTC' });

	const REVIEW: Record<ReviewState, string> = {
		released: 'Released',
		in_review: 'Newer data in review',
		not_released: 'Not released yet'
	};

	let selectedId = $state('');
	const markers = $derived<GlobeMarker[]>(
		data.classrooms
			.filter((c) => c.location)
			.map((c) => ({
				id: c.id,
				lat: c.location!.lat,
				lng: c.location!.lng,
				color: stages.get(c.stageId)?.color ?? '#8e98a3',
				label: `${c.name} · ${stages.get(c.stageId)?.label ?? c.stageId}`
			}))
	);
</script>

{#snippet stageBadge(stageId: string)}
	{@const stage = stages.get(stageId)}
	<span class="chip gap-1.5">
		<span
			class="size-2 rounded-full"
			style:background={stage?.color ?? 'currentColor'}
			aria-hidden="true"
		></span>
		{stage?.label ?? stageId}
	</span>
{/snippet}

{#snippet reading(value: number | null, unit: string)}
	{#if value === null}
		<span class="text-muted italic">no data</span>
	{:else}
		<span class="font-mono whitespace-nowrap tabular-nums">{formatValue(value, unit)}</span>
	{/if}
{/snippet}

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
				<a href="#network" class="btn btn-primary">Explore the data</a>
				{#if host}
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
					<a href={sectionHref(host)} class="btn btn-ghost">Host a Global Classroom</a>
				{/if}
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
		<div class="mt-6 grid gap-6 lg:grid-cols-[1.4fr_1fr]">
			<div class="card relative h-96 overflow-hidden p-0">
				<Globe {markers} bind:selectedId ariaLabel="Globe of Global Classrooms by stage" />
			</div>
			<div class="space-y-4">
				<ul class="space-y-2" aria-label="Global Classrooms">
					{#each data.classrooms as classroom (classroom.id)}
						<li>
							<button
								type="button"
								class={[
									'card flex w-full items-start justify-between gap-3 p-4 text-left transition',
									selectedId === classroom.id ? 'border-ink' : 'hover:border-line-strong'
								]}
								aria-pressed={selectedId === classroom.id}
								disabled={!classroom.location}
								onclick={() => (selectedId = classroom.id)}
							>
								<span class="min-w-0">
									<span class="block font-medium text-ink">{classroom.name}</span>
									<span class="block text-sm text-muted">
										{[classroom.placeName, classroom.country].filter(Boolean).join(', ') ||
											classroom.institutionName}
									</span>
								</span>
								{@render stageBadge(classroom.stageId)}
							</button>
						</li>
					{/each}
				</ul>
				<dl class="space-y-2 text-sm" aria-label="Stages">
					{#each content.stages as stage (stage.id)}
						<div class="flex gap-2">
							<dt class="shrink-0">{@render stageBadge(stage.id)}</dt>
							<dd class="text-muted">{stage.description}</dd>
						</div>
					{/each}
				</dl>
			</div>
		</div>
	</section>

	<section class="mt-16" aria-labelledby="core-title">
		<h2 id="core-title" class="font-display text-3xl">Core readings</h2>
		<p class="mt-2 max-w-2xl text-muted">
			The same few values from every classroom, from released data only. Rain counts the day of each
			classroom's latest released reading.
		</p>
		<div class="card mt-6 overflow-x-auto p-0">
			<table class="w-full min-w-[46rem] text-left text-sm">
				<thead class="text-xs text-muted">
					<tr class="border-b border-line">
						<th class="px-4 py-3 font-medium">Classroom</th>
						<th class="px-4 py-3 text-right font-medium">Rain today</th>
						<th class="px-4 py-3 text-right font-medium">Rain, last 7 days</th>
						<th class="px-4 py-3 text-right font-medium">Air temperature now</th>
						<th class="px-4 py-3 font-medium">Last sync</th>
						<th class="px-4 py-3 font-medium">Review</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-line">
					{#each data.classrooms as { id, name, core } (id)}
						<tr class="align-top">
							<th scope="row" class="px-4 py-3 font-medium text-ink">
								{name}
								{#if core.day}
									<span class="block text-xs font-normal text-muted"
										>{dayFormat.format(new Date(`${core.day}T00:00:00Z`))}</span
									>
								{/if}
							</th>
							<td class="px-4 py-3 text-right"
								>{@render reading(core.rainToday, data.units.rain)}</td
							>
							<td class="px-4 py-3 text-right">
								{@render reading(core.rainLast7Days, data.units.rain)}
								{#if core.rainLast7Days !== null && core.rainDays < 7}
									<span class="block text-xs text-muted">{core.rainDays} of 7 days</span>
								{/if}
							</td>
							<td class="px-4 py-3 text-right"
								>{@render reading(core.airTemperature, data.units.temperature)}</td
							>
							<td class="px-4 py-3 whitespace-nowrap">
								{#if core.lastSync}
									{dateTime.format(new Date(core.lastSync))}
								{:else}
									<span class="text-muted italic">no data</span>
								{/if}
							</td>
							<td class="px-4 py-3">
								<span
									class={[
										'chip',
										core.review === 'released' && 'border-lime/50 text-lime',
										core.review === 'in_review' && 'border-orange/50 text-orange'
									]}>{REVIEW[core.review]}</span
								>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
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
