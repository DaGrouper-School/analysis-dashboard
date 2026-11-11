<script lang="ts">
	import Chart, { type ChartLine } from '#lib/components/Chart.svelte';
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
	// One point a day, at noon, so a day's point sits in the middle of its day.
	const COLOR = 'var(--color-series-1)';
	const at = (day: string) => Date.parse(`${day}T12:00:00.000Z`);
	const span = $derived({
		from: days.length ? at(days[0]) : 0,
		to: days.length ? at(days.at(-1)!) : 0
	});
	function line(id: string, label: string, values: (number | null)[]): ChartLine {
		return {
			id,
			label,
			color: COLOR,
			points: values.flatMap((value, i) => (value === null ? [] : [{ at: at(days[i]), value }]))
		};
	}
	const temperatureLine = $derived<ChartLine>({
		...line(
			'temperature',
			'Average',
			temperature.map((d) => d?.mean ?? null)
		),
		band: temperature.flatMap((d, i) => (d ? [{ at: at(days[i]), low: d.low, high: d.high }] : []))
	});
	const rainBars = $derived([
		{
			id: 'gauge',
			label: 'Gauge',
			color: COLOR,
			values: Object.fromEntries(
				rain.filter((d) => d.value !== null && d.reference).map((d) => [d.day, d.value!])
			)
		},
		{
			id: 'station',
			label: 'Station',
			color: COLOR,
			outlined: true,
			values: Object.fromEntries(
				rain.filter((d) => d.value !== null && !d.reference).map((d) => [d.day, d.value!])
			)
		}
	]);
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
					One point a day from released data. Point at a day, or use the arrow keys, to read it.
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
					<Chart
						label="Air temperature"
						unit={t}
						lines={[temperatureLine]}
						from={span.from}
						to={span.to}
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
					<Chart
						label="Daily rain"
						unit={mm}
						{days}
						bars={rainBars}
						overlay
						flagged={rain.filter((d) => d.flagged).map((d) => d.day)}
					/>
				</figure>
			{/if}
			{#if uvs.length}
				<figure class="card flex min-w-0 flex-col gap-3 p-5">
					<figcaption>
						{@render figure('UV index', "The day's highest", range2(uvs, ''))}
					</figcaption>
					<Chart
						label="UV index"
						lines={[line('uv', 'UV index', uv)]}
						from={span.from}
						to={span.to}
					/>
				</figure>
			{/if}
			{#if soils.length}
				<figure class="card flex min-w-0 flex-col gap-3 p-5">
					<figcaption>
						{@render figure('Soil moisture', "The day's average", range2(soils, soilUnit))}
					</figcaption>
					<Chart
						label="Soil moisture"
						unit={soilUnit}
						lines={[line('soil', 'Soil moisture', soil)]}
						from={span.from}
						to={span.to}
					/>
				</figure>
			{/if}
		</div>
	</section>
{/if}
