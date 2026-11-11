<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ClassValue } from 'svelte/elements';
	import { MediaQuery } from 'svelte/reactivity';

	/**
	 * Content that opens and closes by smoothly growing and shrinking in height. The one
	 * collapsible the app uses: pair it with a button whose `aria-controls` is its `id`. Closed,
	 * it can't be focused or read. `openFrom` keeps it open at and above a breakpoint, such as a
	 * card that is an accordion only on a phone. Reduced motion opens it without animation.
	 */
	let {
		open,
		id,
		openFrom,
		fill = false,
		class: className,
		children
	}: {
		open: boolean;
		id: string;
		/** Always open from this media query up, e.g. `(min-width: 64rem)`. */
		openFrom?: string;
		/** Fills the rest of a flex column, so the content can push its last row to the bottom. */
		fill?: boolean;
		/** Classes for the content, such as its spacing or `flex flex-col`. */
		class?: ClassValue;
		children: Snippet;
	} = $props();

	// svelte-ignore state_referenced_locally
	const wide = openFrom ? new MediaQuery(openFrom, false) : null;
	const shown = $derived(open || !!wide?.current);
</script>

<div {id} class={['collapsible', shown && 'shown', fill && 'flex-1']} inert={!shown}>
	<div class={['min-h-0 overflow-hidden', fill && 'flex flex-col']}>
		<div class={className}>{@render children()}</div>
	</div>
</div>

<style>
	/* Animates from 0 to the content's own height: the row grows from 0fr to 1fr. */
	.collapsible {
		display: grid;
		grid-template-rows: 0fr;
		visibility: hidden;
		transition:
			grid-template-rows 300ms ease,
			visibility 0s 300ms;
	}
	.shown {
		grid-template-rows: 1fr;
		visibility: visible;
		transition:
			grid-template-rows 300ms ease,
			visibility 0s;
	}
	@media (prefers-reduced-motion: reduce) {
		.collapsible,
		.shown {
			transition: none;
		}
	}
</style>
