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
		actions,
		controls,
		onexplain,
		class: className = ''
	}: {
		tiles: NowTile[];
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

<section class={['card flex flex-col', className]} aria-labelledby="now-heading">
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
			> · released data
		{:else}
			No released data yet
		{/if}
	</p>

	<ul class="mt-5 grid grid-cols-2 gap-3 [&>:last-child:nth-child(odd)]:col-span-2">
		{#each tiles as tile (tile.id)}
			<li class="flex flex-col rounded-md border border-line bg-raised p-3.5">
				<p class="flex items-center gap-2 text-sm">
					<Icon name={tile.icon} class="size-4 shrink-0 text-teal" />
					<span class="min-w-0 flex-1 leading-tight">{tile.label}</span>
					{#if onexplain && tile.parameterId}
						{@const id = tile.parameterId}
						<button
							type="button"
							class="-m-1.5 grid size-7 shrink-0 place-items-center rounded-full text-muted hover:text-ink focus-visible:ring-2 focus-visible:ring-teal focus-visible:outline-none"
							aria-label="What {tile.label.toLowerCase()} means and how it is calculated"
							aria-haspopup="dialog"
							onclick={() => onexplain(id)}><Icon name="info" class="size-4" /></button
						>
					{/if}
				</p>
				{#if tile.value === null}
					<p class="mt-3 text-sm text-muted italic">no data</p>
				{:else}
					<p class="mt-2 font-mono text-3xl leading-none text-ink tabular-nums">
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
		<div class="mt-auto flex flex-wrap gap-3 pt-5">{@render actions()}</div>
	{/if}
</section>
