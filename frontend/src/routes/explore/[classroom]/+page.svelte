<script lang="ts">
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import SampleDataBadge from '#lib/components/SampleDataBadge.svelte';
	import StageBadge from '#lib/components/StageBadge.svelte';
	import type { PublicSource, ReviewState } from '#lib/domain/types.ts';
	import { FALLBACK_DOMAIN_ICON } from '#lib/icons.ts';
	import { formatValue } from '#lib/key-values.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const classroom = $derived(data.classroom);
	const core = $derived(classroom.core);
	const stage = $derived(data.content.stages.find((s) => s.id === classroom.stageId));
	const params = $derived(new Map(data.parameters.map((p) => [p.id, p])));
	const explore = $derived(data.content.sections.find((s) => s.id === 'explore'));
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

	const REVIEW: Record<ReviewState, string> = {
		released: 'All data released',
		in_review: 'Newer data in review',
		not_released: 'No data released yet'
	};
	const sourceName = (source: PublicSource) => source.name ?? 'Field observations';
</script>

<svelte:head><title>{classroom.name} · {data.content.hero.eyebrow}</title></svelte:head>

{#snippet stat(label: string, value: number | null, unit: string, note = '')}
	<div class="card">
		<p class="text-xs text-muted">{label}</p>
		<p class="mt-1 text-2xl text-ink">
			{#if value === null}<span class="text-base text-muted italic">no data</span
				>{:else}{formatValue(value, unit)}{/if}
		</p>
		{#if note}<p class="mt-1 text-xs text-muted">{note}</p>{/if}
	</div>
{/snippet}

<PublicShell content={data.content} user={data.user}>
	<nav aria-label="Breadcrumb" class="text-sm text-muted">
		<a class="hover:text-ink" href={resolve('/explore')}>{explore?.label ?? 'Explore data'}</a>
		{#if classroom.country}<span aria-hidden="true" class="px-1">›</span>{classroom.country}{/if}
		<span aria-hidden="true" class="px-1">›</span><span class="text-ink" aria-current="page"
			>{classroom.name}</span
		>
	</nav>

	<header class="mt-4 flex flex-wrap items-start justify-between gap-4">
		<div>
			<div class="flex flex-wrap items-center gap-3">
				<h1 class="font-display text-4xl">{classroom.name}</h1>
				<SampleDataBadge show={data.content.sampleData} />
			</div>
			<p class="mt-2 flex flex-wrap items-center gap-2 text-muted">
				{[classroom.placeName, classroom.country].filter(Boolean).join(', ') ||
					classroom.institutionName}
				<StageBadge {stage} stageId={classroom.stageId} />
			</p>
			<p class="mt-1 text-sm text-muted">
				{core.lastSync ? `Last sync ${dateTime.format(new Date(core.lastSync))}` : 'No sync yet'} ·
				{REVIEW[core.review]}
			</p>
		</div>
		{#if data.member}
			<!-- Members also see raw values and data still in review on their dashboard. -->
			<!-- eslint-disable svelte/no-navigation-without-resolve -->
			<a
				class="btn btn-ghost"
				href={`${resolve('/(app)/dashboard')}?classroom=${encodeURIComponent(classroom.id)}`}
				>Open in your dashboard</a
			>
			<!-- eslint-enable svelte/no-navigation-without-resolve -->
		{/if}
	</header>

	<section class="mt-8" aria-labelledby="core-heading">
		<h2 id="core-heading" class="sr-only">Core readings</h2>
		<div class="grid gap-4 sm:grid-cols-3">
			{@render stat('Rain today', core.rainToday, data.units.rain)}
			{@render stat(
				'Rain, last 7 days',
				core.rainLast7Days,
				data.units.rain,
				core.rainLast7Days !== null && core.rainDays < 7 ? `${core.rainDays} of 7 days` : ''
			)}
			{@render stat('Air temperature now', core.airTemperature, data.units.temperature)}
		</div>
	</section>

	<section class="mt-10" aria-labelledby="domains-heading">
		<h2 id="domains-heading" class="font-display text-2xl">What this classroom measures</h2>
		{#if data.domains.length}
			<div class="mt-4 grid gap-4 md:grid-cols-2">
				{#each data.domains as card (card.domain?.id ?? 'other')}
					<article class="card">
						<h3 class="flex items-center gap-2 text-lg font-semibold">
							<Icon
								name={card.domain?.icon ?? FALLBACK_DOMAIN_ICON}
								fallback={FALLBACK_DOMAIN_ICON}
								class="size-5 text-muted"
							/>
							{card.domain?.label ?? 'Other'}
						</h3>
						{#if card.domain?.description}
							<p class="mt-1 text-sm text-muted">{card.domain.description}</p>
						{/if}
						<ul class="mt-4 divide-y divide-line">
							{#each card.values as value (`${value.source.id}|${value.parameterId}`)}
								{@const param = params.get(value.parameterId)}
								<li class="flex items-start justify-between gap-3 py-2.5">
									<span class="min-w-0">
										<a
											class="link"
											href={resolve('/explore/[classroom]/[parameter]', {
												classroom: classroom.id,
												parameter: value.parameterId
											})}>{param?.label ?? value.parameterId}</a
										>
										<span class="block text-xs text-muted">
											{sourceName(value.source)} · {dateTime.format(new Date(value.observedAt))}
										</span>
									</span>
									<span class="shrink-0 font-mono text-ink tabular-nums">
										{formatValue(value.value, param?.unit, param?.valueType)}
									</span>
								</li>
							{/each}
						</ul>
					</article>
				{/each}
			</div>
		{:else}
			<p class="mt-3 text-muted">Nothing from this classroom has been released yet.</p>
		{/if}
	</section>
</PublicShell>
