<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * One card of the strip under "What's happening now?" (AP-22, AP-45). Every card has the
	 * same rows: a header, the value, a picture of the same height, then a note pinned to the
	 * bottom, so a card with less to say keeps the same shape as its neighbours.
	 */
	let {
		label,
		icon,
		chip,
		value,
		picture,
		note = '',
		href
	}: {
		label: string;
		/** A registry icon beside the label, in the page's colour. */
		icon?: string | null;
		/** Top right, such as the review state. */
		chip?: Snippet;
		value: Snippet;
		picture?: Snippet;
		note?: string;
		/** Opens the value's own page; the whole card is the link. */
		href?: string;
	} = $props();
</script>

<div
	class={[
		'card flex min-h-60 flex-col p-5',
		href && 'relative transition-colors hover:border-line-strong'
	]}
>
	<div class="flex min-h-7 items-center justify-between gap-2">
		<p class="flex items-center gap-2 text-sm text-muted">
			{#if icon}<Icon name={icon} class="size-4 text-(--accent,var(--color-teal))" />{/if}
			{#if href}
				<!-- Callers pass resolved paths. The link covers the card. -->
				<!-- eslint-disable svelte/no-navigation-without-resolve -->
				<a
					class="after:absolute after:inset-0 after:rounded-[inherit] hover:text-ink focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-teal"
					{href}>{label}</a
				>
				<!-- eslint-enable svelte/no-navigation-without-resolve -->
			{:else}
				{label}
			{/if}
		</p>
		{@render chip?.()}
	</div>
	<p class="mt-2 font-mono text-3xl text-ink tabular-nums">{@render value()}</p>
	<div class="mt-4 flex h-16 flex-col justify-center">{@render picture?.()}</div>
	<p class="mt-auto pt-3 text-xs text-muted">{note}</p>
</div>
