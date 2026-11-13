<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * A ⋯ button that opens a short list of actions, for secondary actions that should not compete
	 * with a card's main button. Escape, a click outside or picking an item closes it. Items are
	 * `.menu-item` buttons or links, each with an icon.
	 */
	let {
		label,
		children,
		class: className = ''
	}: {
		/** What the button opens, for screen readers, such as "Device actions". */
		label: string;
		children: Snippet;
		class?: string;
	} = $props();

	let open = $state(false);
	let root = $state<HTMLElement>();
	const id = $props.id();

	function outside(e: PointerEvent) {
		if (open && root && !root.contains(e.target as Node)) open = false;
	}
</script>

<svelte:window
	onpointerdown={outside}
	onkeydown={(e) => {
		if (open && e.key === 'Escape') open = false;
	}}
/>

<div bind:this={root} class={['relative', className]}>
	<button
		type="button"
		class="grid size-10 place-items-center rounded-full border border-line-strong text-muted transition hover:border-ink hover:text-ink focus-visible:ring-2 focus-visible:ring-teal focus-visible:outline-none"
		aria-label={label}
		aria-haspopup="menu"
		aria-expanded={open}
		aria-controls="{id}-menu"
		onclick={() => (open = !open)}
	>
		<Icon name="more" class="size-5" />
	</button>
	{#if open}
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div
			id="{id}-menu"
			role="menu"
			tabindex="-1"
			class="menu absolute top-full right-0 z-30 mt-2 min-w-52 overflow-hidden rounded-md border border-line-strong bg-surface py-1 shadow-2xl"
			onclick={() => (open = false)}
		>
			{@render children()}
		</div>
	{/if}
</div>

<style>
	.menu {
		animation: menu-in 160ms cubic-bezier(0.2, 0.8, 0.2, 1);
		transform-origin: top right;
	}
	.menu :global(.menu-item) {
		display: flex;
		width: 100%;
		align-items: center;
		gap: 0.625rem;
		padding: 0.625rem 1rem;
		font-size: 0.875rem;
		text-align: left;
		color: var(--color-text);
	}
	.menu :global(.menu-item:hover),
	.menu :global(.menu-item:focus-visible) {
		background: var(--color-raised);
		outline: none;
	}
	@keyframes menu-in {
		from {
			opacity: 0;
			transform: translateY(-4px) scale(0.97);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.menu {
			animation: none;
		}
	}
</style>
