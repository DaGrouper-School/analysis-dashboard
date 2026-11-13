<script lang="ts">
	import ActionMenu from '#lib/components/ActionMenu.svelte';
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * The dashboard device's secondary actions, Sync device and Record manually, written once:
	 * side by side buttons on desktop (`buttons`), a ⋯ menu in the card header below it (`menu`).
	 */
	let {
		as,
		onsync,
		recordHref
	}: {
		as: 'buttons' | 'menu';
		/** Opens the sync modal; absent when the user cannot sync this device. */
		onsync?: () => void;
		/** The manual record form for this device; absent when the user cannot record. */
		recordHref?: string;
	} = $props();
</script>

{#snippet items(itemClass: string, role?: 'menuitem')}
	{#if onsync}
		<button type="button" {role} class={itemClass} aria-haspopup="dialog" onclick={onsync}
			><Icon name="sync" class="size-4" /> Sync device</button
		>
	{/if}
	{#if recordHref}
		<!-- eslint-disable svelte/no-navigation-without-resolve -->
		<a {role} class={itemClass} href={recordHref}
			><Icon name="pencil" class="size-4" /> Record manually</a
		>
		<!-- eslint-enable svelte/no-navigation-without-resolve -->
	{/if}
{/snippet}

{#if onsync || recordHref}
	{#if as === 'menu'}
		<ActionMenu label="Device actions" class="-mt-1 -mr-1 lg:hidden">
			{@render items('menu-item', 'menuitem')}
		</ActionMenu>
	{:else}
		{@render items('btn btn-ghost flex-1 rounded-full px-3 whitespace-nowrap max-lg:hidden')}
	{/if}
{/if}
