<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Sparkline from '#lib/components/Sparkline.svelte';
	import type { NowTile } from '#lib/domain/types.ts';
	import { formatValue } from '#lib/key-values.ts';
	import { timeAgo } from '#lib/station-now.ts';

	/**
	 * "What's happening now?" (AP-43): a few current values as large tiles with a plain line
	 * each, and when the newest of them was measured. Missing data reads "no data", never zero.
	 * With `onexplain`, each value has an info button for how it is calculated (AP-07).
	 */
	let {
		tiles,
		released = true,
		accent,
		actions,
		controls,
		onexplain,
		class: className = ''
	}: {
		tiles: NowTile[];
		/** Released data only (the public view); false for a device's own latest readings. */
		released?: boolean;
		/** Tints the card, its icons and lines, such as the picked device's registry colour. */
		accent?: string;
		/** Buttons under the tiles, such as "View domains". */
		actions?: Snippet;
		/** Beside the heading, such as the device picker. */
		controls?: Snippet;
		/** Opens what a value means and how it is calculated. */
		onexplain?: (parameterId: string) => void;
		class?: string;
	} = $props();

	const latest = $derived(
		tiles
			.map((t) => t.observedAt)
			.filter((at): at is string => !!at)
			.sort()
			.at(-1) ?? null
	);
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

	/** The value without its unit, which the tile sets smaller beside it. */
	function bare(tile: NowTile) {
		return typeof tile.value === 'number' ? formatValue(tile.value) : String(tile.value);
	}
</script>

<section
	class={['card flex flex-col', accent && 'accent-panel', className]}
	style:--accent={accent}
	aria-labelledby="now-heading"
>
	<div class="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
		<h2 id="now-heading" class="text-xl font-medium">What's happening now?</h2>
		{@render controls?.()}
	</div>
	<p class="text-xs text-muted">
		{#if latest}
			Last update
			<time
				class="font-semibold text-text"
				datetime={latest}
				title={dateTime.format(new Date(latest))}>{timeAgo(latest)}</time
			>{released ? ' · released data' : ''}
		{:else}
			{released ? 'No released data yet' : 'No readings yet'}
		{/if}
	</p>

	<!-- The tiles share the card's height evenly, with the same gap between them every way. -->
	<ul
		class="mt-5 grid flex-1 auto-rows-fr grid-cols-2 gap-3 [&>:last-child:nth-child(odd)]:col-span-2"
	>
		{#each tiles as tile (tile.id)}
			<li
				class={[
					'flex min-w-0 flex-col rounded-md border p-4',
					accent ? 'accent-tile' : 'border-line bg-raised'
				]}
			>
				<!-- The icon and the info button on one line, the label under them at full width. -->
				<div class="flex items-center justify-between gap-2">
					<Icon name={tile.icon} class="size-4 shrink-0 text-(--accent,var(--color-teal))" />
					{#if onexplain && tile.parameterId}
						{@const id = tile.parameterId}
						<button
							type="button"
							class="grid size-5 shrink-0 place-items-center rounded-full text-muted hover:text-ink focus-visible:ring-2 focus-visible:ring-teal focus-visible:outline-none"
							aria-label="What {tile.label.toLowerCase()} means and how it is calculated"
							aria-haspopup="dialog"
							onclick={() => onexplain(id)}><Icon name="info" class="size-4" /></button
						>
					{/if}
				</div>
				<p class="mt-2 text-sm leading-5">{tile.label}</p>
				{#if tile.value === null}
					<p class="mt-3 text-sm text-muted italic">no data</p>
				{:else}
					<p class="mt-3 font-mono text-3xl leading-none text-ink tabular-nums">
						{bare(tile)}<span class="ml-1 text-sm text-muted">{tile.unit}</span>
					</p>
				{/if}
				{#if tile.caption}<p class="mt-2 text-xs text-muted">{tile.caption}</p>{/if}
				<div class="mt-auto pt-3"><Sparkline fill values={tile.last24Hours} /></div>
			</li>
		{/each}
	</ul>
	{#if tiles.some((t) => t.last24Hours.some((v) => v !== null))}
		<p class="mt-3 text-xs text-muted">Lines show the last 24 hours.</p>
	{/if}

	{#if actions}
		<div class="flex flex-wrap gap-3 pt-5">{@render actions()}</div>
	{/if}
</section>
