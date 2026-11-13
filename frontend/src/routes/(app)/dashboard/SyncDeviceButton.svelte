<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * Sync device in What's happening now, written once: a full button above View domains on
	 * desktop (`button`), just its icon beside the card's heading below desktop (`icon`).
	 */
	let {
		as,
		onsync
	}: {
		as: 'button' | 'icon';
		/** Opens the sync modal. */
		onsync: () => void;
	} = $props();
</script>

<button
	type="button"
	class={[
		'btn btn-ghost rounded-full',
		as === 'icon' ? '-mt-1 size-10 shrink-0 p-0 lg:hidden' : 'w-full max-lg:hidden'
	]}
	aria-label={as === 'icon' ? 'Sync device' : undefined}
	aria-haspopup="dialog"
	onclick={onsync}
	><Icon name="sync" class={as === 'icon' ? 'size-5' : 'size-4'} />{#if as === 'button'}
		Sync device{/if}</button
>
