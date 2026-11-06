<script lang="ts">
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import ReadingChart from '#lib/components/ReadingChart.svelte';
	import SampleDataBadge from '#lib/components/SampleDataBadge.svelte';
	import type { PublicSource } from '#lib/domain/types.ts';
	import { FALLBACK_DOMAIN_ICON } from '#lib/icons.ts';
	import { formatValue } from '#lib/key-values.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const parameter = $derived(data.parameter);
	const numeric = $derived((parameter.valueType ?? 'number') === 'number');
	const explore = $derived(data.content.sections.find((s) => s.id === 'explore'));
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const sourceName = (source: PublicSource) => source.name ?? 'Field observations';
	/** The chart reads readings; each point becomes one with just this value. */
	const asReadings = (points: { observedAt: string; value: number | string }[]) =>
		points.map((p) => ({ observedAt: p.observedAt, values: { [parameter.id]: p.value } }));
</script>

<svelte:head
	><title>{parameter.label} · {data.classroom.name} · {data.content.hero.eyebrow}</title
	></svelte:head
>

<PublicShell content={data.content} user={data.user}>
	<nav aria-label="Breadcrumb" class="text-sm text-muted">
		<a class="hover:text-ink" href={resolve('/explore')}>{explore?.label ?? 'Explore data'}</a>
		<span aria-hidden="true" class="px-1">›</span>
		<a
			class="hover:text-ink"
			href={resolve('/explore/[classroom]', { classroom: data.classroom.id })}
			>{data.classroom.name}</a
		>
		<span aria-hidden="true" class="px-1">›</span><span class="text-ink" aria-current="page"
			>{parameter.label}</span
		>
	</nav>

	<header class="mt-4">
		<div class="flex flex-wrap items-center gap-3">
			<h1 class="font-display text-4xl">{parameter.label}</h1>
			<SampleDataBadge show={data.content.sampleData} />
		</div>
		{#if data.domain}
			<p class="mt-2 flex items-center gap-1.5 text-sm text-muted">
				<Icon name={data.domain.icon} fallback={FALLBACK_DOMAIN_ICON} class="size-4" />
				{data.domain.label}
			</p>
		{/if}
		<p class="mt-3 max-w-2xl">{parameter.plain ?? parameter.definition}</p>
		{#if parameter.plain && parameter.definition !== parameter.plain}
			<p class="mt-1 max-w-2xl text-sm text-muted">{parameter.definition}</p>
		{/if}
	</header>

	<div class="mt-8 space-y-6">
		{#each data.series as series (series.source.id)}
			{@const latest = series.points.at(-1)}
			<section class="card" aria-labelledby="series-{series.source.id}">
				<div class="flex flex-wrap items-baseline justify-between gap-3">
					<h2 id="series-{series.source.id}" class="font-semibold">{sourceName(series.source)}</h2>
					{#if latest}
						<p class="text-sm text-muted">
							Latest <span class="font-mono text-lg text-ink tabular-nums"
								>{formatValue(latest.value, parameter.unit, parameter.valueType)}</span
							>
							· {dateTime.format(new Date(latest.observedAt))}
						</p>
					{/if}
				</div>
				{#if numeric}
					<div class="mt-4">
						<ReadingChart
							readings={asReadings(series.points)}
							parameterId={parameter.id}
							label={parameter.label}
							unit={parameter.unit}
						/>
					</div>
				{:else}
					<ul class="mt-3 divide-y divide-line text-sm">
						{#each series.points.slice(-10).reverse() as point (point.observedAt)}
							<li class="flex justify-between gap-3 py-2">
								<span class="text-muted">{dateTime.format(new Date(point.observedAt))}</span>
								<span class="text-ink"
									>{formatValue(point.value, parameter.unit, parameter.valueType)}</span
								>
							</li>
						{/each}
					</ul>
				{/if}
			</section>
		{/each}
	</div>
	<p class="mt-4 text-sm text-muted">Released values only. Newer values appear once reviewed.</p>
</PublicShell>
