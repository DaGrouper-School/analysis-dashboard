<script lang="ts">
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import { sensorQuery } from '#lib/sensor-selection.ts';
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import Chart, { type ChartBars, type ChartLine } from '#lib/components/Chart.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import SampleDataBadge from '#lib/components/SampleDataBadge.svelte';
	import type { PublicParameterPage, PublicParameterSeries } from '#lib/domain/types.ts';
	import { FALLBACK_DOMAIN_ICON } from '#lib/icons.ts';
	import { formatValue } from '#lib/key-values.ts';
	import {
		chartOf,
		dailyStats,
		dayOf,
		daysOf,
		inRange,
		latestOf,
		parameterCsv,
		rainDays,
		RANGES,
		type RangeId
	} from '#lib/parameter-page.ts';
	import { UTF8_BOM } from '#lib/raw-data.ts';
	import { errorMessage, services } from '#lib/services/index.ts';
	import { sectionHref } from '#lib/site.ts';
	import type { PageProps } from './$types';

	/**
	 * One value of one classroom (AP-23), always in the same order: what it means here, the value
	 * now, the main chart (compare in its header), the daily summary (download in its header), data
	 * quality, and how it is measured (the curriculum link in its header). Released values only.
	 */
	let { data }: PageProps = $props();

	const parameter = $derived(data.parameter);
	const chart = $derived(chartOf(parameter));
	const unit = $derived(parameter.unit);
	const directory = $derived(data.content.sections.find((s) => s.id === 'classrooms'));
	const learn = $derived(data.content.sections.find((s) => s.id === 'learn'));
	const COLORS = ['var(--color-series-1)', 'var(--color-series-2)'];
	const COMPARE_COLOR = 'var(--color-series-3)';

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const dayLong = new Intl.DateTimeFormat(undefined, {
		weekday: 'short',
		day: 'numeric',
		month: 'short',
		timeZone: 'UTC'
	});
	const dayText = (day: string) => dayLong.format(Date.parse(`${day}T12:00:00.000Z`));
	const show = (value: number | string | null | undefined) =>
		value === null || value === undefined ? '–' : formatValue(value, unit, parameter.valueType);

	const sourceName = (s: Pick<PublicParameterSeries, 'source'>) =>
		s.source.name ?? 'Field observations';
	const isReference = (s: PublicParameterSeries) => data.rain?.referenceSourceId === s.source.id;
	const seriesLabel = (s: PublicParameterSeries) =>
		isReference(s) ? `${sourceName(s)} (reference)` : sourceName(s);

	let range = $state<RangeId>('7d');
	/** Values other than rain show one source at a time; rain shows every source together. */
	let sourceId = $state<string | null>(null);
	const series = $derived(
		data.series.find((s) => s.source.id === sourceId) ?? data.series[0] ?? null
	);
	const latest = $derived(latestOf(data.series));
	const rangeDays = $derived(
		latest ? daysOf(dayOf(latest), RANGES.find((r) => r.id === range)!.days) : []
	);

	// ---- Rain: every source by day, the gauge as reference ----
	const rain = $derived(
		data.rain && chart === 'daily_bars'
			? rainDays(data.series, data.rain.referenceSourceId, data.rain.tolerance)
			: null
	);
	const rainInRange = $derived(
		rain?.filter((d) => d.day >= rangeDays[0] && d.day <= rangeDays.at(-1)!) ?? []
	);

	// ---- Other numbers: one source, by day ----
	const stats = $derived(series ? dailyStats(series.points) : []);
	const statsInRange = $derived(stats.filter((d) => d.day >= (rangeDays[0] ?? '')));

	// ---- Compare with another classroom ----
	let compareId = $state('');
	let compared = $state<PublicParameterPage | null>(null);
	let compareError = $state('');
	$effect(() => {
		const id = compareId;
		compared = null;
		compareError = '';
		if (!id) return;
		services.publicSite
			.parameter(id, parameter.id)
			.then((page) => {
				if (compareId === id) compared = page;
			})
			.catch((e) => (compareError = errorMessage(e)));
	});
	/** The other classroom's line or bars: its reference gauge, or its first source. */
	const comparedSeries = $derived(
		compared
			? (compared.series.find((s) => s.source.id === compared!.rain?.referenceSourceId) ??
					compared.series[0])
			: null
	);
	const comparedRain = $derived(
		compared?.rain ? rainDays(compared.series, compared.rain.referenceSourceId, 0) : null
	);
	const comparedStats = $derived(comparedSeries ? dailyStats(comparedSeries.points) : []);

	// ---- The chart's data for the chosen range ----
	const chartBars = $derived.by((): ChartBars[] => {
		if (!rain) return [];
		const result: ChartBars[] = data.series.map((s, i) => ({
			id: s.source.id,
			label: seriesLabel(s),
			color: COLORS[i % COLORS.length],
			values: Object.fromEntries(
				rainInRange
					.filter((d) => d.totals[s.source.id] !== undefined)
					.map((d) => [d.day, d.totals[s.source.id]])
			)
		}));
		if (compared && comparedRain) {
			result.push({
				id: 'compare',
				label: compared.classroom.name,
				color: COMPARE_COLOR,
				values: Object.fromEntries(
					comparedRain
						.filter((d) => d.shown !== null && rangeDays.includes(d.day))
						.map((d) => [d.day, d.shown!])
				)
			});
		}
		return result;
	});

	const window = $derived.by(() => {
		if (!latest) return { from: 0, to: 0 };
		const to = Date.parse(latest);
		return range === '24h'
			? { from: to - 86_400_000, to }
			: { from: Date.parse(`${rangeDays[0]}T00:00:00.000Z`), to };
	});
	const noon = (day: string) => Date.parse(`${day}T12:00:00.000Z`);

	function lineOf(
		s: PublicParameterSeries,
		dayStats: ReturnType<typeof dailyStats>,
		base: Omit<ChartLine, 'points' | 'band'>
	): ChartLine {
		const numbers = s.points.filter(
			(p): p is { observedAt: string; value: number } => typeof p.value === 'number'
		);
		if (range === '24h' || chart === 'line') {
			const points = latest ? inRange(numbers, latest, range) : [];
			return {
				...base,
				points: points
					.map((p) => ({ at: Date.parse(p.observedAt), value: p.value }))
					.filter((p) => p.at >= window.from)
			};
		}
		const days = dayStats.filter((d) => rangeDays.includes(d.day));
		return {
			...base,
			points: days.map((d) => ({ at: noon(d.day), value: d.mean })),
			band: base.dashed
				? undefined
				: days.map((d) => ({ at: noon(d.day), low: d.low, high: d.high }))
		};
	}

	const chartLines = $derived.by((): ChartLine[] => {
		if (rain || !series || !chart) return [];
		const lines = [
			lineOf(series, stats, { id: series.source.id, label: data.classroom.name, color: COLORS[0] })
		];
		if (compared && comparedSeries) {
			lines.push(
				lineOf(comparedSeries, comparedStats, {
					id: 'compare',
					label: compared.classroom.name,
					color: COMPARE_COLOR,
					dashed: true
				})
			);
		}
		return lines;
	});

	const legend = $derived(
		rain
			? chartBars.map((b) => ({ label: b.label, color: b.color, dashed: false }))
			: chartLines.length > 1
				? chartLines.map((l) => ({ label: l.label, color: l.color, dashed: !!l.dashed }))
				: []
	);

	// ---- Value now ----
	const now = $derived.by(() => {
		if (rain) {
			const day = rain.at(-1);
			if (!day) return null;
			const from = data.series.find((s) => s.source.id === day.shownFrom);
			return {
				value: show(day.shown),
				caption: `Rain on ${dayText(day.day)}${from ? `, from ${sourceName(from)}` : ''}`,
				review: day.review
			};
		}
		const point = series?.points.at(-1);
		if (!point || !series) return null;
		const today = stats.at(-1);
		return {
			value: show(point.value),
			caption: `${sourceName(series)}, ${dateTime.format(new Date(point.observedAt))}`,
			range:
				chart === 'line_band' && today && today.day === dayOf(point.observedAt) && today.count > 1
					? `Today ${show(today.low)} to ${show(today.high)}`
					: null,
			review: false
		};
	});

	// ---- Data quality ----
	const quality = $derived.by(() => {
		const days30 = latest ? daysOf(dayOf(latest), 30) : [];
		const withData = new Set(
			(rain ? rain.map((d) => d.day) : stats.map((d) => d.day)).filter((d) => days30.includes(d))
		);
		return {
			received: withData.size,
			expected: days30.length,
			flagged: rain?.filter((d) => d.review && days30.includes(d.day)).length ?? 0
		};
	});

	// ---- Download ----
	function download() {
		const units = Object.fromEntries(data.series.map((s) => [s.parameterId, unit]));
		const csv = parameterCsv(data.classroom.id, data.series, units, data.terms);
		const url = URL.createObjectURL(new Blob([UTF8_BOM, csv], { type: 'text/csv;charset=utf-8' }));
		const link = Object.assign(document.createElement('a'), {
			href: url,
			download: `dagrouper-${data.classroom.id}-${parameter.id}-released.csv`
		});
		link.click();
		setTimeout(() => URL.revokeObjectURL(url), 1000);
	}

	const METHOD = {
		measured: 'Measured',
		derived: 'Calculated from other values',
		manual: 'Entered by hand'
	};
	const SOURCE = {
		weather_station: 'A weather station',
		device: 'A sensor',
		manual: 'People, on a guided form'
	};
</script>

<svelte:head
	><title>{parameter.label} · {data.classroom.name} · {data.content.hero.eyebrow}</title
	></svelte:head
>

<PublicShell content={data.content} user={data.user}>
	<!-- A member goes back to the classroom on their dashboard, not to its public page. -->
	<Breadcrumbs
		items={data.member
			? [
					{ label: 'Dashboard', href: resolve('/(app)/dashboard') },
					{
						label: data.classroom.name,
						href: resolve('/(app)/dashboard') + sensorQuery(data.classroom.id)
					},
					{ label: parameter.label }
				]
			: [
					{ label: directory?.label ?? 'Classrooms', href: resolve('/classrooms') },
					{
						label: data.classroom.name,
						href: resolve('/classrooms/[classroom]', { classroom: data.classroom.id })
					},
					{ label: parameter.label }
				]}
	/>

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
	</header>

	<!-- 1. What it means here -->
	<section class="mt-6 max-w-2xl" aria-labelledby="meaning-heading">
		<h2 id="meaning-heading" class="text-sm font-medium text-muted">What it means here</h2>
		<p class="mt-1 text-lg">{data.note ?? parameter.plain ?? parameter.definition}</p>
		{#if data.note}
			<p class="mt-2 text-sm text-muted">
				Written by {data.classroom.name}. {parameter.plain ?? parameter.definition}
			</p>
		{/if}
	</section>

	<!-- 2. Value now -->
	{#if now}
		<section class="mt-6" aria-labelledby="now-heading">
			<h2 id="now-heading" class="sr-only">Value now</h2>
			<p class="font-mono text-5xl text-ink tabular-nums">{now.value}</p>
			<p class="mt-1 text-sm text-muted">{now.caption}</p>
			{#if 'range' in now && now.range}<p class="text-sm text-muted">{now.range}</p>{/if}
			{#if now.review}
				<p class="mt-2 inline-flex items-center gap-1.5 text-sm text-orange">
					<Icon name="alert" class="size-4" /> The sources disagree on this day: flagged for review.
				</p>
			{/if}
		</section>
	{/if}

	<!-- 3. Main chart, with compare -->
	<section class="card mt-6" aria-labelledby="chart-heading">
		<div class="flex flex-wrap items-center justify-between gap-3">
			<h2 id="chart-heading" class="font-semibold">
				{rain
					? 'Daily rain'
					: chart === 'line_band'
						? 'Readings, with each day’s low to high'
						: 'Readings'}
			</h2>
			<div class="flex flex-wrap items-center gap-2">
				{#if data.compare.length}
					<select
						class="input w-auto max-w-72 truncate py-1 pr-8 text-sm"
						aria-label="Compare with another classroom"
						bind:value={compareId}
					>
						<option value="">Compare with…</option>
						{#each data.compare as c (c.id)}
							<option value={c.id}>{c.name}{c.placeName ? `, ${c.placeName}` : ''}</option>
						{/each}
					</select>
				{/if}
				{#if chart}
					<div
						class="flex rounded-lg border border-line p-0.5 text-sm"
						role="group"
						aria-label="Time range"
					>
						{#each RANGES as r (r.id)}
							<button
								type="button"
								class={[
									'rounded-md px-3 py-1',
									range === r.id ? 'bg-raised text-ink' : 'text-muted hover:text-ink'
								]}
								aria-pressed={range === r.id}
								onclick={() => (range = r.id)}>{r.label}</button
							>
						{/each}
					</div>
				{/if}
			</div>
		</div>
		{#if compareError}<p class="mt-2 text-sm text-danger" role="alert">{compareError}</p>{/if}
		{#if !rain && data.series.length > 1}
			<label class="mt-3 flex items-center gap-2 text-sm text-muted">
				Source
				<select class="input w-auto py-1" bind:value={sourceId}>
					{#each data.series as s (s.source.id)}
						<option value={s.source.id}>{sourceName(s)}</option>
					{/each}
				</select>
			</label>
		{/if}
		{#if legend.length}
			<ul class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-sm text-text">
				{#each legend as item (item.label)}
					<li class="flex items-center gap-1.5">
						{#if item.dashed}
							<span
								class="w-4 border-t-2 border-dashed"
								style:border-color={item.color}
								aria-hidden="true"
							></span>
						{:else}
							<span class="size-2.5 rounded-sm" style:background={item.color} aria-hidden="true"
							></span>
						{/if}
						{item.label}
					</li>
				{/each}
			</ul>
		{/if}
		<div class="mt-4">
			{#if rain}
				<Chart
					label={parameter.label}
					{unit}
					days={range === '24h' ? rangeDays.slice(-1) : rangeDays}
					bars={chartBars}
					flagged={rainInRange.filter((d) => d.review).map((d) => d.day)}
				/>
			{:else if chart}
				<Chart
					label={parameter.label}
					{unit}
					lines={chartLines}
					from={window.from}
					to={window.to}
					hourly={range === '24h'}
				/>
			{:else if series}
				<ul class="divide-y divide-line text-sm">
					{#each series.points.slice(-10).reverse() as point (point.observedAt)}
						<li class="flex justify-between gap-3 py-2">
							<span class="text-muted">{dateTime.format(new Date(point.observedAt))}</span>
							<span class="text-ink">{show(point.value)}</span>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
		{#if rain && data.rain}
			<p class="mt-3 text-sm text-muted">
				{#if data.rain.referenceSourceId}
					The manual gauge is the reference. When another source is more than {show(
						data.rain.tolerance
					)} away from it, the day is flagged for review. Sources are never averaged.
				{:else}
					No manual gauge here, so each day shows the station’s total.
				{/if}
			</p>
		{/if}
	</section>

	<!-- 4. Daily summary, with download -->
	<section class="mt-8" aria-labelledby="daily-heading">
		<div class="flex flex-wrap items-center justify-between gap-3">
			<h2 id="daily-heading" class="font-display text-2xl">Daily summary</h2>
			<button type="button" class="btn btn-ghost" onclick={download}>
				<Icon name="download" class="size-4" /> Download CSV
			</button>
		</div>
		{#if chart}
			<div class="card mt-3 overflow-x-auto p-0">
				<table class="w-full min-w-[32rem] text-left text-sm">
					<thead class="text-xs text-muted">
						<tr class="border-b border-line">
							<th class="px-4 py-3 font-medium">Day</th>
							{#if rain}
								{#each data.series as s (s.source.id)}
									<th class="px-4 py-3 text-right font-medium">{seriesLabel(s)}</th>
								{/each}
								<th class="px-4 py-3 font-medium">Check</th>
							{:else}
								<th class="px-4 py-3 text-right font-medium">Low</th>
								<th class="px-4 py-3 text-right font-medium">Average</th>
								<th class="px-4 py-3 text-right font-medium">High</th>
								<th class="px-4 py-3 text-right font-medium">Readings</th>
							{/if}
						</tr>
					</thead>
					<tbody class="divide-y divide-line">
						{#if rain}
							{#each rainInRange.toReversed() as day (day.day)}
								<tr>
									<td class="px-4 py-2.5 whitespace-nowrap">{dayText(day.day)}</td>
									{#each data.series as s (s.source.id)}
										<td class="px-4 py-2.5 text-right font-mono tabular-nums"
											>{show(day.totals[s.source.id])}</td
										>
									{/each}
									<td class="px-4 py-2.5">
										{#if day.review}
											<span class="text-orange">Flagged for review, {show(day.spread)} apart</span>
										{:else if day.spread !== null}
											<span class="text-muted">Agree</span>
										{:else}
											<span class="text-muted">One source</span>
										{/if}
									</td>
								</tr>
							{:else}
								<tr
									><td class="px-4 py-3 text-muted" colspan="9"
										>No released readings in this range.</td
									></tr
								>
							{/each}
						{:else}
							{#each statsInRange.toReversed() as day (day.day)}
								<tr>
									<td class="px-4 py-2.5 whitespace-nowrap">{dayText(day.day)}</td>
									<td class="px-4 py-2.5 text-right font-mono tabular-nums">{show(day.low)}</td>
									<td class="px-4 py-2.5 text-right font-mono tabular-nums">{show(day.mean)}</td>
									<td class="px-4 py-2.5 text-right font-mono tabular-nums">{show(day.high)}</td>
									<td class="px-4 py-2.5 text-right tabular-nums">{day.count}</td>
								</tr>
							{:else}
								<tr
									><td class="px-4 py-3 text-muted" colspan="5"
										>No released readings in this range.</td
									></tr
								>
							{/each}
						{/if}
					</tbody>
				</table>
			</div>
		{/if}
		<p class="mt-2 text-xs text-muted">
			The CSV carries the licence ({data.terms.licence}) and how to credit {data.classroom.name}.
		</p>
	</section>

	<!-- 5. Data quality -->
	<section class="mt-8" aria-labelledby="quality-heading">
		<h2 id="quality-heading" class="font-display text-2xl">Data quality</h2>
		<dl class="mt-3 grid gap-3 sm:grid-cols-3">
			<div class="card">
				<dt class="text-sm text-muted">Days with readings, last 30</dt>
				<dd class="mt-1 font-mono text-2xl text-ink tabular-nums">
					{quality.received}<span class="text-base text-muted"> / {quality.expected}</span>
				</dd>
			</div>
			<div class="card">
				<dt class="text-sm text-muted">Reviewed and released through</dt>
				<dd class="mt-1 text-ink">
					{data.releasedThrough ? dateTime.format(new Date(data.releasedThrough)) : 'Nothing yet'}
				</dd>
				{#if data.inReview}<dd class="mt-1 text-sm text-muted">
						Newer readings are in review.
					</dd>{/if}
			</div>
			{#if rain}
				<div class="card">
					<dt class="text-sm text-muted">Days flagged for review, last 30</dt>
					<dd
						class={[
							'mt-1 font-mono text-2xl tabular-nums',
							quality.flagged ? 'text-orange' : 'text-ink'
						]}
					>
						{quality.flagged}
					</dd>
				</div>
			{:else}
				<div class="card">
					<dt class="text-sm text-muted">Sources</dt>
					<dd class="mt-1 text-ink">{data.series.map(sourceName).join(', ')}</dd>
				</div>
			{/if}
		</dl>
	</section>

	<!-- 6. How it is measured, with the curriculum link -->
	<section class="mt-8" aria-labelledby="measured-heading">
		<div class="flex flex-wrap items-center justify-between gap-3">
			<h2 id="measured-heading" class="font-display text-2xl">How it is measured</h2>
			{#if learn}
				<!-- eslint-disable svelte/no-navigation-without-resolve -- sectionHref resolves -->
				<a class="link inline-flex items-center gap-1.5 text-sm" href={sectionHref(learn)}
					>Learn about it <Icon name="arrow" class="size-4" /></a
				>
				<!-- eslint-enable svelte/no-navigation-without-resolve -->
			{/if}
		</div>
		<dl class="mt-3 max-w-2xl space-y-2 text-sm">
			<div class="flex gap-2">
				<dt class="w-32 shrink-0 text-muted">Taken by</dt>
				<dd>
					{#if rain}
						{data.series
							.map((s) => (isReference(s) ? `${sourceName(s)}, read by hand` : sourceName(s)))
							.join('; ')}
					{:else}
						{SOURCE[parameter.source]}{data.deviceTypes.length
							? ` (${data.deviceTypes.join(', ')})`
							: ''}
					{/if}
				</dd>
			</div>
			{#if parameter.calculation}
				<div class="flex gap-2">
					<dt class="w-32 shrink-0 text-muted">Method</dt>
					<dd>{METHOD[parameter.calculation.method]}: {parameter.calculation.formula}</dd>
				</div>
				<div class="flex gap-2">
					<dt class="w-32 shrink-0 text-muted">Registry</dt>
					<dd>SAM registry {parameter.calculation.version}</dd>
				</div>
			{/if}
			<div class="flex gap-2">
				<dt class="w-32 shrink-0 text-muted">Unit</dt>
				<dd>
					{unit || 'None'}{parameter.min !== undefined && parameter.max !== undefined
						? `, accepted from ${parameter.min} to ${parameter.max}`
						: ''}
				</dd>
			</div>
		</dl>
	</section>
</PublicShell>
