<script lang="ts">
	import DailyChart from '#lib/components/DailyChart.svelte';
	import type { PublicStation } from '#lib/domain/types.ts';
	import { formatValue } from '#lib/key-values.ts';

	/**
	 * The classroom charts (AP-45): air temperature, daily rain, UV and soil moisture over the
	 * last 7 or 30 days, one point a day from released data. The same on the public classroom
	 * page and the dashboard. A chart with no data at all is left out.
	 */
	let { station, class: className = '' }: { station: PublicStation; class?: string } = $props();

	let range = $state<7 | 30>(30);
	const charts = $derived(station.charts);
	const from = $derived(Math.max(0, charts.days.length - range));
	const days = $derived(charts.days.slice(from));
	const temperature = $derived(charts.temperature.slice(from));
	const rain = $derived(charts.rain.slice(from));
	const uv = $derived(charts.uv.slice(from));
	const soil = $derived(charts.soilMoisture.slice(from));
	const unitOf = (id: string, fallback: string) =>
		station.parameters.find((p) => p.id === id)?.unit ?? fallback;
	const t = $derived(station.units.temperature);
	const mm = $derived(station.units.rain);
	const soilUnit = $derived(unitOf('soil_moisture', '%'));

	const numbers = (values: (number | null)[]) => values.filter((v): v is number => v !== null);
	const temps = $derived(numbers(temperature.map((d) => d?.mean ?? null)));
	const rainTotal = $derived(numbers(rain.map((d) => d.value)).reduce((a, b) => a + b, 0));
	const flagged = $derived(rain.filter((d) => d.flagged).length);
	const stationDays = $derived(rain.filter((d) => d.value !== null && !d.reference).length);
	const soils = $derived(numbers(soil));
	const uvs = $derived(numbers(uv));
	const range2 = (values: number[], unit: string) =>
		values.length
			? `${formatValue(Math.min(...values))} to ${formatValue(Math.max(...values), unit)}`
			: 'no data';
</script>

{#snippet figure(title: string, sub: string, stat: string)}
	<span class="flex flex-col gap-0.5">
		<span class="flex items-baseline justify-between gap-3">
			<span class="font-medium text-ink">{title}</span>
			<span class="font-mono text-sm whitespace-nowrap text-ink">{stat}</span>
		</span>
		<span class="text-sm text-muted">{sub}</span>
	</span>
{/snippet}

{#snippet key(kind: 'line' | 'band' | 'bar' | 'outline' | 'flag', text: string)}
	<span class="flex items-center gap-1.5">
		{#if kind === 'line'}<span class="h-0.5 w-4 rounded-full bg-series-1"></span>
		{:else if kind === 'band'}<span class="h-2.5 w-3.5 rounded-sm bg-series-1/25"></span>
		{:else if kind === 'bar'}<span class="h-3 w-2.5 rounded-sm bg-series-1"></span>
		{:else if kind === 'outline'}<span
				class="h-3 w-2.5 rounded-sm border-[1.5px] border-dashed border-series-1"
			></span>
		{:else}<span class="border-x-[5px] border-b-[8px] border-x-transparent border-b-orange"
			></span>{/if}
		{text}
	</span>
{/snippet}

{#if charts.days.length}
	<section class={className} aria-labelledby="charts-heading">
		<div class="flex flex-wrap items-end justify-between gap-3">
			<div>
				<h2 id="charts-heading" class="font-display text-2xl">Charts</h2>
				<p class="mt-1 max-w-2xl text-sm text-muted">
					One point a day from released data. Point at a day to see its value.
				</p>
			</div>
			<div
				role="group"
				aria-label="Time range"
				class="inline-flex gap-1 rounded-full border border-line-strong p-1"
			>
				{#each [7, 30] as const as option (option)}
					<button
						type="button"
						aria-pressed={range === option}
						class={[
							'min-h-9 rounded-full px-4 text-sm font-medium transition-colors',
							range === option ? 'bg-ink text-canvas' : 'text-muted hover:text-ink'
						]}
						onclick={() => (range = option)}>{option} days</button
					>
				{/each}
			</div>
		</div>

		<div class="mt-5 grid gap-3 lg:grid-cols-2">
			{#if temps.length}
				<figure class="card flex min-w-0 flex-col gap-3 p-5">
					<figcaption>
						{@render figure(
							'Air temperature',
							"The day's average, with its low to high",
							range2(temps, t)
						)}
					</figcaption>
					<div class="flex flex-wrap gap-x-4 gap-y-1 text-xs">
						{@render key('line', 'Average')}
						{@render key('band', 'Low to high')}
					</div>
					<DailyChart
						{days}
						line={temperature.map((d) => d?.mean ?? null)}
						band={temperature}
						label="Air temperature, last {range} days"
						tick={(v) => `${formatValue(v)}°`}
						tip={(i) =>
							temperature[i]
								? `${formatValue(temperature[i]!.mean, t)} · ${formatValue(temperature[i]!.low)} to ${formatValue(temperature[i]!.high)}`
								: null}
					/>
				</figure>
			{/if}
			{#if rain.some((d) => d.value !== null)}
				<figure class="card flex min-w-0 flex-col gap-3 p-5">
					<figcaption>
						{@render figure(
							'Daily rain',
							`One total a day${stationDays ? `, ${stationDays} from the station` : ''}${flagged ? `, ${flagged} flagged for review` : ''}.`,
							`${formatValue(Math.round(rainTotal * 10) / 10, mm)} in total`
						)}
					</figcaption>
					<div class="flex flex-wrap gap-x-4 gap-y-1 text-xs">
						{@render key('bar', 'Gauge, the reference')}
						{@render key('outline', 'Station, no gauge reading')}
						{#if flagged}{@render key('flag', 'Flagged for review')}{/if}
					</div>
					<DailyChart
						{days}
						bars={rain}
						label="Daily rain, last {range} days"
						tick={(v) => formatValue(v)}
						tip={(i) =>
							rain[i].value === null
								? null
								: `${formatValue(rain[i].value!, mm)}, ${rain[i].reference ? 'gauge' : 'station'}${rain[i].flagged ? ' · flagged' : ''}`}
					/>
				</figure>
			{/if}
			{#if uvs.length}
				<figure class="card flex min-w-0 flex-col gap-3 p-5">
					<figcaption>
						{@render figure('UV index', "The day's highest", range2(uvs, ''))}
					</figcaption>
					<DailyChart
						{days}
						line={uv}
						label="UV index, last {range} days"
						tick={(v) => formatValue(v)}
						tip={(i) => (uv[i] === null ? null : `UV ${formatValue(uv[i]!)}`)}
					/>
				</figure>
			{/if}
			{#if soils.length}
				<figure class="card flex min-w-0 flex-col gap-3 p-5">
					<figcaption>
						{@render figure('Soil moisture', "The day's average", range2(soils, soilUnit))}
					</figcaption>
					<DailyChart
						{days}
						line={soil}
						label="Soil moisture, last {range} days"
						tick={(v) => `${formatValue(v)}${soilUnit}`}
						tip={(i) => (soil[i] === null ? null : formatValue(soil[i]!, soilUnit))}
					/>
				</figure>
			{/if}
		</div>
	</section>
{/if}
