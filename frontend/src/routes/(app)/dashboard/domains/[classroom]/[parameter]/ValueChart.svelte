<script lang="ts">
	import Chart from '#lib/components/Chart.svelte';
	import Select from '#lib/components/Select.svelte';
	import type {
		Chart as ChartKind,
		Parameter,
		PublicParameterPage,
		PublicParameterSeries
	} from '#lib/domain/types.ts';
	import {
		dailyStats,
		rainDays,
		RANGES,
		type RangeId
	} from '#lib/features/measurements/parameter-page.ts';
	import type { ComponentProps, Snippet } from 'svelte';
	import {
		COMPARE_COLOR,
		SERIES_COLORS,
		chartWindow,
		comparedSeriesOf,
		legendOf,
		rainBars,
		seriesLine
	} from './chart-data.ts';
	import { dateTime, showValue, sourceName } from './value-format.ts';

	/**
	 * The main chart (AP-23), its kind set in the registry: daily bars for rain from every source
	 * (the gauge as reference), otherwise one source as a line. Compare and the time range sit in
	 * its header; the alert levels come in under it as `levelsEditor`.
	 */
	let {
		page,
		parameter,
		chart,
		latest,
		rangeDays,
		rain,
		rainInRange,
		series,
		stats,
		compared,
		compareError,
		levels,
		range = $bindable(),
		compareId = $bindable(),
		sourceId = $bindable(),
		levelsEditor
	}: {
		page: Pick<PublicParameterPage, 'classroom' | 'series' | 'rain' | 'compare'>;
		parameter: Parameter;
		chart: ChartKind | null;
		latest: string | null;
		rangeDays: string[];
		rain: ReturnType<typeof rainDays> | null;
		rainInRange: ReturnType<typeof rainDays>;
		/** The source shown when the value is not rain. */
		series: PublicParameterSeries | null;
		stats: ReturnType<typeof dailyStats>;
		compared: PublicParameterPage | null;
		compareError: string;
		levels: ComponentProps<typeof Chart>['thresholds'];
		range: RangeId;
		compareId: string;
		sourceId: string | null;
		/** The alert levels for the device shown, under the chart. */
		levelsEditor?: Snippet;
	} = $props();

	const show = (value: number | string | null | undefined) => showValue(value, parameter);
	const referenceId = $derived(page.rain?.referenceSourceId ?? null);
	const window = $derived(chartWindow(latest, range, rangeDays));

	const comparedSeries = $derived(compared ? comparedSeriesOf(compared) : null);
	const bars = $derived(
		rain
			? rainBars(
					page.series,
					referenceId,
					rainInRange,
					rangeDays,
					compared?.rain
						? {
								name: compared.classroom.name,
								days: rainDays(compared.series, compared.rain.referenceSourceId, 0)
							}
						: null
				)
			: []
	);
	const lines = $derived.by(() => {
		if (rain || !series || !chart) return [];
		const context = { range, chart, latest, rangeDays, window };
		const result = [
			seriesLine(
				series,
				stats,
				{
					id: series.source.id,
					label: page.classroom.name,
					color: series.source.color ?? SERIES_COLORS[0]
				},
				context
			)
		];
		if (compared && comparedSeries) {
			result.push(
				seriesLine(
					comparedSeries,
					dailyStats(comparedSeries.points),
					{ id: 'compare', label: compared.classroom.name, color: COMPARE_COLOR, dashed: true },
					context
				)
			);
		}
		return result;
	});
	const legend = $derived(legendOf(!!rain, bars, lines));
</script>

<section class="card mt-6" aria-labelledby="chart-heading">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<h2 id="chart-heading" class="font-semibold">
			{rain
				? 'Daily rain'
				: range === '24h'
					? 'Readings, last 24 hours'
					: chart === 'line_band'
						? 'Readings, with each day’s low to high'
						: 'Readings'}
		</h2>
		<div class="flex flex-wrap items-center gap-2">
			{#if page.compare.length}
				<Select
					size="compact"
					align="end"
					class="max-w-72"
					label="Compare with"
					options={[
						{ value: '', label: 'Compare with…' },
						...page.compare.map((c) => ({
							value: c.id,
							label: c.name,
							hint: c.placeName || undefined
						}))
					]}
					bind:value={compareId}
				/>
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
	{#if !rain && page.series.length > 1}
		<label class="mt-3 flex items-center gap-2 text-sm text-muted">
			Source
			<Select
				size="compact"
				options={page.series.map((s) => ({ value: s.source.id, label: sourceName(s) }))}
				bind:value={sourceId}
			/>
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
				unit={parameter.unit}
				days={range === '24h' ? rangeDays.slice(-1) : rangeDays}
				{bars}
				flagged={rainInRange.filter((d) => d.review).map((d) => d.day)}
			/>
		{:else if chart}
			<Chart
				label={parameter.label}
				unit={parameter.unit}
				{lines}
				from={window.from}
				to={window.to}
				hourly={range === '24h'}
				thresholds={levels}
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
	{@render levelsEditor?.()}
	{#if rain && page.rain}
		<p class="mt-3 text-sm text-muted">
			{#if page.rain.referenceSourceId}
				The manual gauge is the reference. When another source is more than {show(
					page.rain.tolerance
				)} away from it, the day is flagged for review. Sources are never averaged.
			{:else}
				No manual gauge here, so each day shows the station’s total.
			{/if}
		</p>
	{/if}
</section>
