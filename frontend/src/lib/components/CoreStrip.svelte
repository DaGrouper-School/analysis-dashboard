<script lang="ts">
	import ReviewChip from '#lib/components/ReviewChip.svelte';
	import type { PublicStation } from '#lib/domain/types.ts';
	import { formatValue } from '#lib/key-values.ts';

	/**
	 * The core readings (AP-22) as three cards, the same on the public classroom page and the
	 * dashboard: rain today by source against the gauge, rain on each of the last 7 days, and
	 * today's air temperature range. Released data only; anything missing reads "no data".
	 */
	let { station, class: className = '' }: { station: PublicStation; class?: string } = $props();

	const core = $derived(station.classroom.core);
	const rain = $derived(station.core.rain);
	const temp = $derived(station.core.temperature);
	const units = $derived(station.units);

	// Rain today: one bar per source, on a scale that fits the largest.
	const rainMax = $derived(Math.max(1, ...rain.bySource.map((s) => s.value)));
	const sourceLabel = (s: (typeof rain.bySource)[number]) =>
		s.name ?? (s.sourceId === 'field' ? 'Field observations' : s.sourceId);
	const rainNote = $derived.by(() => {
		if (rain.bySource.length < 2 || rain.spread === null) return '';
		const gauge = rain.bySource.some((s) => s.reference) ? 'The gauge is the reference. ' : '';
		const within = rain.spread <= rain.tolerance ? 'within' : 'more than';
		return `${gauge}${formatValue(rain.spread, units.rain)} apart, ${within} the ${formatValue(rain.tolerance, units.rain)} the protocol allows.`;
	});

	// Rain, last 7 days: a bar a day, gauge solid, station outlined, flagged days marked.
	const weekMax = $derived(Math.max(1, ...rain.week.map((d) => d.value ?? 0)));
	const weekday = new Intl.DateTimeFormat(undefined, { weekday: 'short', timeZone: 'UTC' });
	const gaugeDays = $derived(rain.week.filter((d) => d.value !== null && d.reference).length);
	const stationDays = $derived(rain.week.filter((d) => d.value !== null && !d.reference).length);
	const flaggedDays = $derived(rain.week.filter((d) => d.flagged).length);

	// Air temperature: today's low to high on the last 30 days' range, with now marked.
	const pos = (v: number) =>
		temp.monthLow === null || temp.monthHigh === null || temp.monthHigh === temp.monthLow
			? 50
			: ((v - temp.monthLow) / (temp.monthHigh - temp.monthLow)) * 100;
</script>

{#snippet noData()}
	<span class="font-sans text-base text-muted italic">no data</span>
{/snippet}

<section class={['grid gap-3 md:grid-cols-3', className]} aria-labelledby="core-heading">
	<h2 id="core-heading" class="sr-only">Core readings</h2>

	<!-- A steady height, so a classroom with less to say does not move what is below. -->
	<div class="card flex min-h-52 flex-col gap-3 p-5">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<p class="text-sm text-muted">Rain today</p>
			<ReviewChip state={rain.review} />
		</div>
		<p class="font-mono text-3xl text-ink tabular-nums">
			{#if core.rainToday === null}{@render noData()}{:else}{formatValue(core.rainToday)}<span
					class="ml-1 text-sm text-muted">{units.rain}</span
				>{/if}
		</p>
		{#if rain.bySource.length > 1}
			<ul class="space-y-2 text-sm">
				{#each rain.bySource as source (source.sourceId)}
					<li class="grid grid-cols-[minmax(0,9rem)_1fr_auto] items-center gap-3">
						<span class="truncate" title={sourceLabel(source)}
							>{source.reference ? 'Gauge' : sourceLabel(source)}</span
						>
						<span class="h-2 overflow-hidden rounded-full bg-raised" aria-hidden="true">
							<span
								class={[
									'block h-full rounded-full',
									source.reference ? 'bg-series-1' : 'bg-series-2'
								]}
								style:width="{(source.value / rainMax) * 100}%"
							></span>
						</span>
						<span class="font-mono text-ink tabular-nums"
							>{formatValue(source.value, units.rain)}</span
						>
					</li>
				{/each}
			</ul>
		{/if}
		{#if rainNote}<p class="text-xs text-muted">{rainNote}</p>{/if}
	</div>

	<div class="card flex min-h-52 flex-col gap-3 p-5">
		<p class="text-sm text-muted">Rain, last 7 days</p>
		<p class="font-mono text-3xl text-ink tabular-nums">
			{#if core.rainLast7Days === null}{@render noData()}{:else}{formatValue(
					core.rainLast7Days
				)}<span class="ml-1 text-sm text-muted">{units.rain}</span>{/if}
		</p>
		{#if rain.week.length}
			<ol class="grid grid-cols-7 items-end gap-1.5" aria-label="Rain on each of the last 7 days">
				{#each rain.week as day (day.day)}
					{@const label = weekday.format(new Date(`${day.day}T00:00:00Z`))}
					<li
						class="flex flex-col items-center gap-1"
						title="{label}: {day.value === null
							? 'no reading'
							: `${formatValue(day.value, units.rain)}, ${day.reference ? 'gauge' : 'station'}`}{day.flagged
							? ', flagged for review'
							: ''}"
					>
						<span class="flex h-12 w-full items-end justify-center" aria-hidden="true">
							{#if day.value === null}
								<span class="h-px w-4 bg-line-strong"></span>
							{:else if day.value === 0}
								<span class="h-0.5 w-5 rounded-full bg-muted/60"></span>
							{:else}
								<span
									class={[
										'relative w-5 rounded-t-[4px]',
										day.reference
											? 'bg-series-1'
											: 'border-[1.5px] border-dashed border-series-1 bg-transparent'
									]}
									style:height="{Math.max(10, (day.value / weekMax) * 100)}%"
								>
									{#if day.flagged}
										<span
											class="absolute -top-3.5 left-1/2 -translate-x-1/2 border-x-[5px] border-b-[8px] border-x-transparent border-b-orange"
										></span>
									{/if}
								</span>
							{/if}
						</span>
						<span class="text-xs text-muted">{label}</span>
						<span class="sr-only"
							>{day.value === null ? 'no reading' : formatValue(day.value, units.rain)}</span
						>
					</li>
				{/each}
			</ol>
			<p class="text-xs text-muted">
				{gaugeDays
					? `Gauge on ${gaugeDays} day${gaugeDays === 1 ? '' : 's'}.${stationDays ? ` On ${stationDays} day${stationDays === 1 ? '' : 's'} with no gauge reading, the station.` : ''}`
					: stationDays
						? 'All from the station: no gauge reading this week.'
						: ''}{flaggedDays ? ` ${flaggedDays} flagged for review.` : ''}
			</p>
		{/if}
	</div>

	<div class="card flex min-h-52 flex-col gap-3 p-5">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<p class="text-sm text-muted">Air temperature today</p>
			<ReviewChip state={temp.review} />
		</div>
		<p class="font-mono text-3xl text-ink tabular-nums">
			{#if temp.low === null || temp.high === null}{@render noData()}{:else}{formatValue(temp.low)} to
				{formatValue(temp.high)}<span class="ml-1 text-sm text-muted">{units.temperature}</span
				>{/if}
		</p>
		{#if temp.low !== null && temp.high !== null}
			<div class="mt-1 px-3" aria-hidden="true">
				<div class="relative h-2 rounded-full bg-raised">
					<span
						class="absolute inset-y-0 rounded-full bg-series-1"
						style:left="{pos(temp.low)}%"
						style:width="{Math.max(2, pos(temp.high) - pos(temp.low))}%"
					></span>
					{#if core.airTemperature !== null}
						<span
							class="absolute -top-1 h-4 w-1 -translate-x-1/2 rounded-full bg-ink"
							style:left="{pos(core.airTemperature)}%"
						></span>
					{/if}
				</div>
				<div class="relative mt-2 h-4 font-mono text-xs text-muted">
					<span class="absolute -translate-x-1/2" style:left="{pos(temp.low)}%"
						>{formatValue(temp.low)}</span
					>
					<span class="absolute -translate-x-1/2" style:left="{pos(temp.high)}%"
						>{formatValue(temp.high)}</span
					>
				</div>
			</div>
			<p class="text-xs text-muted">
				{core.airTemperature !== null
					? `Now ${formatValue(core.airTemperature, units.temperature)}.`
					: ''}
				{temp.monthLow !== null && temp.monthHigh !== null
					? `The last 30 days ran from ${formatValue(temp.monthLow, units.temperature)} to ${formatValue(temp.monthHigh, units.temperature)}.`
					: ''}
			</p>
		{/if}
	</div>
</section>
