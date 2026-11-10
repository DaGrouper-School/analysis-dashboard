<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';
	import type { NowTile } from '#lib/domain/types.ts';
	import { formatValue } from '#lib/key-values.ts';
	import { timeAgo } from '#lib/station-now.ts';

	/**
	 * "What's happening now?" (AP-43): a few current values as large tiles with a plain line
	 * each, and when the newest of them was measured. Missing data reads "no data", never zero.
	 */
	let {
		tiles,
		actions,
		class: className = ''
	}: {
		tiles: NowTile[];
		/** Buttons under the tiles, such as "View domains". */
		actions?: Snippet;
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
	<h2 id="now-heading" class="text-xl font-medium">What's happening now?</h2>
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

	<ul class="mt-5 grid grid-cols-2 gap-3">
		{#each tiles as tile (tile.id)}
			<li class="rounded-md border border-line bg-raised p-3.5">
				<p class="flex items-center gap-2 text-sm">
					<Icon name={tile.icon} class="size-4 text-teal" />
					{tile.label}
				</p>
				{#if tile.value === null}
					<p class="mt-3 text-sm text-muted italic">no data</p>
				{:else}
					<p class="mt-2 font-mono text-3xl leading-none text-ink tabular-nums">
						{bare(tile)}<span class="ml-1 text-sm text-muted">{tile.unit}</span>
					</p>
				{/if}
				{#if tile.caption}<p class="mt-2 text-xs text-muted">{tile.caption}</p>{/if}
			</li>
		{/each}
	</ul>

	{#if actions}
		<div class="mt-auto flex flex-wrap gap-3 pt-5">{@render actions()}</div>
	{/if}
</section>
