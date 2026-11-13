<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * A registry icon in a small square tinted with its colour, such as a device type's: the same
	 * look in dropdowns and on cards, so a colour always reads as the same sensor.
	 */
	let {
		name,
		color,
		class: className = ''
	}: {
		name: string;
		/** Registry colour; muted without one. */
		color?: string | null;
		class?: string;
	} = $props();
</script>

<span class={['color-icon', className]} style:--accent={color || undefined} aria-hidden="true">
	<Icon {name} class="size-[1em]" />
</span>

<style>
	.color-icon {
		display: grid;
		width: 1.625rem;
		height: 1.625rem;
		flex: none;
		place-items: center;
		border: 1px solid color-mix(in oklab, var(--accent, var(--color-muted)) 40%, var(--color-line));
		border-radius: 0.3rem;
		background: color-mix(in oklab, var(--accent, var(--color-muted)) 12%, var(--color-raised));
		color: var(--accent, var(--color-text));
		font-size: 1rem;
	}
</style>
