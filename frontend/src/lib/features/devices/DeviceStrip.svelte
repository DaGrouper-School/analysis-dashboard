<script lang="ts">
	import StripCard from '#lib/features/devices/StripCard.svelte';
	import type { DeviceSummaryCard } from '#lib/domain/types.ts';
	import { formatValue } from '#lib/features/measurements/key-values.ts';

	/**
	 * The picked device's card numbers (set in the Registry) under "What's happening now?" (AP-45): a card each with
	 * the newest value, the last 7 days as bars and today's and the week's range. Which values,
	 * and their order, are set per device type in the Registry. The same on the public classroom
	 * page and the dashboard; drawn in the page's colour.
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

	function note(card: DeviceSummaryCard) {
		const unit = card.unit;
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
		<StripCard label={card.label} icon={card.icon} note={note(card)} href={href?.(card.id)}>
			{#snippet value()}
				{#if card.latest === null}
					<span class="font-sans text-base text-muted italic">no data</span>
				{:else}
					{formatValue(card.latest)}<span class="ml-1 text-sm text-muted">{card.unit}</span>
				{/if}
			{/snippet}
			{#snippet picture()}
				<ol
					class="grid h-16 grid-cols-7 items-end gap-1.5"
					aria-label="{card.label} on each of the last 7 days"
				>
					{#each card.week as day (day.day)}
						{@const label = weekday.format(new Date(`${day.day}T00:00:00Z`))}
						<li
							class="flex flex-col items-center gap-1"
							title="{label}: {day.value === null
								? 'no reading'
								: formatValue(day.value, card.unit)}"
						>
							<span class="flex h-10 w-full items-end justify-center" aria-hidden="true">
								{#if day.value === null}
									<span class="h-px w-4 bg-line-strong"></span>
								{:else}
									<span
										class="w-5 rounded-t-[4px] bg-(--accent,var(--color-series-1))"
										style:height="{height(card, day.value)}%"
									></span>
								{/if}
							</span>
							<span class="text-xs text-muted">{label}</span>
							<span class="sr-only"
								>{day.value === null ? 'no reading' : formatValue(day.value, card.unit)}</span
							>
						</li>
					{/each}
				</ol>
			{/snippet}
		</StripCard>
	{/each}
</section>
