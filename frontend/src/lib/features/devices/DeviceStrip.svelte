<script lang="ts">
	import ReviewChip from '#lib/components/ReviewChip.svelte';
	import RollingNumber from '#lib/components/RollingNumber.svelte';
	import StripCard from '#lib/features/devices/StripCard.svelte';
	import type { DeviceSummaryCard } from '#lib/domain/types.ts';
	import { formatValue } from '#lib/features/measurements/key-values.ts';

	/**
	 * The picked device's card numbers (set in the Registry) under "What's happening now?" (AP-45): a card each with
	 * the newest value, the last 7 days and today's and the week's range, and where the value
	 * stands in review. The week is drawn by the value's chart kind in the Registry: daily bars
	 * up from zero for totals such as rain; each day's low to high for values such as air
	 * temperature, whose card leads with today's range; else each day's last reading. Which
	 * values, and their order, are set per device type in the Registry. The same on the public
	 * classroom page and the dashboard; drawn in the page's colour.
	 */
	let {
		cards,
		href,
		class: className = ''
	}: {
		cards: DeviceSummaryCard[];
		/** Each card's value page. */
		href?: (parameterId: string) => string;
		class?: string;
	} = $props();

	const weekday = new Intl.DateTimeFormat(undefined, { weekday: 'short', timeZone: 'UTC' });

	/** A bar's height in its card: the week's range, with a floor so the lowest still shows. */
	function height(card: DeviceSummaryCard, value: number) {
		const low = card.weekLow ?? 0;
		const high = card.weekHigh ?? 0;
		return high === low ? 60 : 12 + ((value - low) / (high - low)) * 88;
	}

	/** Daily bars stand on zero, so the days' totals compare. */
	function total(card: DeviceSummaryCard, value: number) {
		const top = Math.max(0, ...card.week.map((d) => d.value ?? 0));
		return top > 0 ? Math.max(8, (Math.max(0, value) / top) * 100) : 0;
	}

	/** Where a value sits on the week's low to high, from the bottom, in percent. */
	function at(card: DeviceSummaryCard, value: number) {
		const low = card.weekLow ?? 0;
		const high = card.weekHigh ?? 0;
		return high === low ? 50 : ((value - low) / (high - low)) * 100;
	}

	/** A card with a range chart leads with today's low to high. */
	const showsRange = (card: DeviceSummaryCard) =>
		card.chart === 'line_band' && card.todayLow !== null && card.todayHigh !== null;

	function note(card: DeviceSummaryCard) {
		const unit = card.unit;
		if (showsRange(card)) {
			return [
				card.latest !== null ? `Now ${formatValue(card.latest, unit)}.` : '',
				card.weekLow !== null && card.weekHigh !== null
					? `The last 7 days ran from ${formatValue(card.weekLow, unit)} to ${formatValue(card.weekHigh, unit)}.`
					: ''
			]
				.filter(Boolean)
				.join(' ');
		}
		return [
			card.todayLow !== null && card.todayHigh !== null && card.todayLow !== card.todayHigh
				? `Today ${formatValue(card.todayLow, unit)} to ${formatValue(card.todayHigh, unit)}.`
				: '',
			card.weekLow !== null && card.weekHigh !== null
				? `The last 7 days ran from ${formatValue(card.weekLow, unit)} to ${formatValue(card.weekHigh, unit)}.`
				: ''
		]
			.filter(Boolean)
			.join(' ');
	}
</script>

<section
	class={[
		'grid gap-3',
		cards.length > 1 && 'md:grid-cols-2',
		cards.length > 2 && 'lg:grid-cols-3',
		className
	]}
	aria-labelledby="device-strip-heading"
>
	<h2 id="device-strip-heading" class="col-span-full pt-6 text-xl font-medium">Last week</h2>
	{#each cards as card (card.id)}
		<StripCard
			label={showsRange(card) ? `${card.label} today` : card.label}
			icon={card.icon}
			note={note(card)}
			href={href?.(card.id)}
		>
			{#snippet chip()}<ReviewChip state={card.review} />{/snippet}
			{#snippet value()}
				{#if card.latest === null}
					<span class="font-sans text-base text-muted italic">no data</span>
				{:else if showsRange(card)}
					<RollingNumber value={formatValue(card.todayLow!)} /> to
					<RollingNumber value={formatValue(card.todayHigh!)} /><span
						class="ml-1 text-sm text-muted">{card.unit}</span
					>
				{:else}
					<RollingNumber value={formatValue(card.latest)} /><span class="ml-1 text-sm text-muted"
						>{card.unit}</span
					>
				{/if}
			{/snippet}
			{#snippet picture()}
				<ol
					class="grid h-16 grid-cols-7 items-end gap-1.5"
					aria-label="{card.label} on each of the last 7 days"
				>
					{#each card.week as day (day.day)}
						{@const label = weekday.format(new Date(`${day.day}T00:00:00Z`))}
						{@const band = card.chart === 'line_band' && day.low !== null && day.high !== null}
						{@const text =
							day.value === null
								? 'no reading'
								: band
									? `${formatValue(day.low!, card.unit)} to ${formatValue(day.high!, card.unit)}`
									: formatValue(day.value, card.unit)}
						<li class="flex flex-col items-center gap-1" title="{label}: {text}">
							<span class="relative flex h-10 w-full items-end justify-center" aria-hidden="true">
								{#if day.value === null}
									<span class="h-px w-4 bg-line-strong"></span>
								{:else if band}
									<!-- The day's low to high on the week's range. -->
									<span
										class="absolute w-2 rounded-full bg-(--accent,var(--color-series-1))"
										style:bottom="{at(card, day.low!)}%"
										style:height="max(0.375rem, {at(card, day.high!) - at(card, day.low!)}%)"
									></span>
								{:else if card.chart === 'daily_bars'}
									{#if day.value === 0}
										<span class="h-0.5 w-5 rounded-full bg-muted/60"></span>
									{:else}
										<span
											class="w-5 rounded-t-[4px] bg-(--accent,var(--color-series-1))"
											style:height="{total(card, day.value)}%"
										></span>
									{/if}
								{:else}
									<span
										class="w-5 rounded-t-[4px] bg-(--accent,var(--color-series-1))"
										style:height="{height(card, day.value)}%"
									></span>
								{/if}
							</span>
							<span class="text-xs text-muted">{label}</span>
							<span class="sr-only">{text}</span>
						</li>
					{/each}
				</ol>
			{/snippet}
		</StripCard>
	{/each}
</section>
