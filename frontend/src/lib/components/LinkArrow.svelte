<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * A link's text with an arrow after it, on a card or tile that opens a page, so a reader can
	 * tell it is a link. The arrow stays on the line of the last word, and moves a little when
	 * the card (`group`) is hovered.
	 */
	let { text }: { text: string } = $props();

	const cut = $derived(text.lastIndexOf(' '));
</script>

{text.slice(0, cut + 1)}<span class="whitespace-nowrap"
	>{text.slice(cut + 1)}<Icon
		name="arrow"
		class="ml-1 inline size-3.5 align-[-0.125em] text-muted transition-transform group-hover:translate-x-0.5 group-hover:text-ink motion-reduce:transition-none"
	/></span
>
