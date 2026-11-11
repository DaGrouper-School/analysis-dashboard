<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';
	import RollingNumber from '#lib/components/RollingNumber.svelte';
	import Sparkline from '#lib/components/Sparkline.svelte';
	import type { NowTile } from '#lib/domain/types.ts';
	import { formatValue } from '#lib/features/measurements/key-values.ts';
	import { timeAgo } from '#lib/features/devices/station-now.ts';

	/**
	 * "What's happening now?" (AP-43): a few current values as large tiles with a plain line
	 * each, and when the newest of them was measured. Missing data reads "no data", never zero.
	 * With `onexplain`, each value has an info button for how it is calculated (AP-07). Values
	 * roll to their new digits when they change, and each says which way it went (AP-07).
	 */
	let {
		tiles,
		released = true,
		shared = true,
		accent,
		icon,
		actions,
		onexplain,
		href,
		element = $bindable(),
		class: className = ''
	}: {
		tiles: NowTile[];
		/** Released data only (the public view); false for a device's own latest readings. */
		released?: boolean;
		/** The classroom shows its data publicly; off, visitors are told so rather than "not yet". */
		shared?: boolean;
		/** Tints the card, its icons and lines, such as the picked device's registry colour. */
		accent?: string;
		/** The source's registry icon, shown when it has nothing to show yet. */
		icon?: string;
		/** Buttons under the tiles, such as "View domains". */
		actions?: Snippet;
		/** Opens what a value means and how it is calculated. */
		onexplain?: (parameterId: string) => void;
		/** Each tile's value page; the whole tile is the link. */
		href?: (parameterId: string) => string;
		/** The card itself, so a page can bring it into view. */
		element?: HTMLElement;
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
	bind:this={element}
	class={['card flex scroll-mt-24 flex-col', accent && 'accent-panel', className]}
	style:--accent={accent}
	aria-labelledby="now-heading"
>
	<div class="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
		<h2 id="now-heading" class="text-xl font-medium">What's happening now?</h2>
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
			{!released ? 'No readings yet' : shared ? 'No released data yet' : 'Not shared publicly'}
		{/if}
	</p>

	{#if !tiles.length}
		<!-- Nothing to show yet: say so, rather than fill the card with another source's values. -->
		<div
			class="mt-5 grid flex-1 place-items-center rounded-md border border-dashed border-line-strong p-6 text-center"
		>
			<div>
				{#if icon}<Icon name={icon} class="mx-auto size-6 text-(--accent,var(--color-teal))" />{/if}
				<p class={['font-medium text-ink', icon && 'mt-3']}>No results yet</p>
				<p class="mt-1 text-sm text-muted">
					{!released
						? 'This device has not sent any readings yet.'
						: shared
							? 'Nothing from this device has been released yet.'
							: 'This classroom keeps its data to its members.'}
				</p>
			</div>
		</div>
	{:else}
		<!-- Three or more tiles share the card's height evenly, with the same gap between them every
		     way. One row of tiles keeps its own height, rather than stretching down the card. -->
		<ul
			class={[
				'mt-5 grid grid-cols-2 gap-3 [&>:last-child:nth-child(odd)]:col-span-2',
				tiles.length > 2 ? 'flex-1 auto-rows-fr' : 'content-start'
			]}
		>
			<!-- Keyed by position, so picking another device rolls the numbers in place. -->
			{#each tiles as tile, i (i)}
				<li
					class={[
						'relative flex min-w-0 flex-col rounded-md border p-4',
						accent ? 'accent-tile' : 'border-line bg-raised',
						href && tile.parameterId && 'transition-colors hover:border-line-strong'
					]}
				>
					<!-- The icon and the info button on one line, the label under them at full width. -->
					<div class="flex items-center justify-between gap-2">
						<Icon name={tile.icon} class="size-4 shrink-0 text-(--accent,var(--color-teal))" />
						{#if onexplain && tile.parameterId}
							{@const id = tile.parameterId}
							<button
								type="button"
								class="relative z-10 grid size-5 shrink-0 place-items-center rounded-full text-muted hover:text-ink focus-visible:ring-2 focus-visible:ring-teal focus-visible:outline-none"
								aria-label="What {tile.label.toLowerCase()} means and how it is calculated"
								aria-haspopup="dialog"
								onclick={() => onexplain(id)}><Icon name="info" class="size-4" /></button
							>
						{/if}
					</div>
					<p class="mt-2 text-sm leading-5">
						{#if href && tile.parameterId}
							<!-- Callers pass resolved paths. The link covers the tile. -->
							<!-- eslint-disable svelte/no-navigation-without-resolve -->
							<a
								class="after:absolute after:inset-0 after:rounded-[inherit] focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-teal"
								href={href(tile.parameterId)}>{tile.label}</a
							>
							<!-- eslint-enable svelte/no-navigation-without-resolve -->
						{:else}
							{tile.label}
						{/if}
					</p>
					{#if tile.value === null}
						<p class="mt-3 text-sm text-muted italic">no data</p>
					{:else}
						<p class="mt-3 font-mono text-3xl leading-none text-ink tabular-nums">
							<RollingNumber value={bare(tile)} /><span class="ml-1 text-sm text-muted"
								>{tile.unit}</span
							>
						</p>
					{/if}
					{#if tile.trend}
						<p class="mt-2 text-xs text-muted" title={tile.trend.text}>
							<span aria-hidden="true">{tile.trend.label}</span>
							<span class="sr-only">{tile.trend.text}</span>
						</p>
					{/if}
					{#if tile.caption}<p class="mt-2 text-xs text-muted">{tile.caption}</p>{/if}
					<div class="mt-auto pt-3"><Sparkline fill values={tile.last24Hours} /></div>
				</li>
			{/each}
		</ul>
	{/if}
	{#if tiles.some((t) => t.last24Hours.some((v) => v !== null))}
		<p class="mt-3 text-xs text-muted">Lines show the last 24 hours.</p>
	{/if}

	{#if actions}
		<div class="mt-auto flex flex-wrap gap-3 pt-5">{@render actions()}</div>
	{/if}
</section>
