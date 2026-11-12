<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ClassValue } from 'svelte/elements';
	import Collapse from '#lib/components/Collapse.svelte';
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * A card that opens and closes: the one accordion card the app uses. Tapping anywhere on its
	 * header toggles it, the chevron turns, and the body grows and shrinks smoothly (`Collapse`).
	 * `openFrom: 'lg'` makes it an accordion on a phone only: always open on a wide screen, with
	 * no toggle there. Links or buttons in the header's `aside` stay clickable above it.
	 */
	let {
		id,
		open = $bindable(false),
		label,
		heading,
		sub,
		aside,
		openFrom,
		fill = false,
		compact = false,
		ontoggle,
		class: className,
		bodyClass,
		children
	}: {
		id: string;
		open?: boolean;
		/** What the card is, for the toggle's label: "Open the {label}". */
		label: string;
		/** The header's title, as an `h2` with id `{id}-heading`. */
		heading: Snippet;
		/** A line under the title. */
		sub?: string;
		/** Beside the title, before the chevron, such as a link or a position. */
		aside?: Snippet;
		/** Always open on this breakpoint and up. */
		openFrom?: 'lg';
		/** The body fills the rest of a flex column, so it can push its last row down. */
		fill?: boolean;
		/** A small card, such as a note under a form, with a small title. */
		compact?: boolean;
		/** Runs after a tap opens or closes it, with the new state. */
		ontoggle?: (open: boolean) => void;
		class?: ClassValue;
		bodyClass?: ClassValue;
		children: Snippet;
	} = $props();

	function toggle() {
		open = !open;
		ontoggle?.(open);
	}
</script>

<section
	class={['card flex flex-col', compact && 'px-4 py-3 text-sm text-muted', className]}
	aria-labelledby="{id}-heading"
>
	<div class="relative flex flex-wrap items-center justify-between gap-x-4 gap-y-1">
		<!-- The title keeps 12rem before the aside wraps under it. -->
		<div class="min-w-0 flex-1 basis-48">
			<h2
				id="{id}-heading"
				class={['flex items-center gap-2 font-medium', compact ? 'text-sm text-ink' : 'text-xl']}
			>
				{@render heading()}
			</h2>
			{#if sub}<p class="text-xs text-muted">{sub}</p>{/if}
		</div>
		<div class="flex shrink-0 items-center gap-3">
			{#if aside}<div class="relative z-10 flex items-center">{@render aside()}</div>{/if}
			<!-- The whole header opens and closes the card. -->
			<button
				type="button"
				class={[
					'grid size-8 shrink-0 place-items-center rounded-full text-muted after:absolute after:inset-0 after:rounded-md hover:text-ink focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-teal',
					openFrom === 'lg' && 'lg:hidden'
				]}
				aria-expanded={open}
				aria-controls="{id}-body"
				aria-label="{open ? 'Close' : 'Open'} the {label}"
				onclick={toggle}
			>
				<Icon name="chevron" class="size-5 transition-transform {open ? 'rotate-180' : ''}" />
			</button>
		</div>
	</div>
	<Collapse
		id="{id}-body"
		{open}
		openFrom={openFrom === 'lg' ? '(min-width: 64rem)' : undefined}
		{fill}
		class={bodyClass}
	>
		{@render children()}
	</Collapse>
</section>
