<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import type { RawQuery } from '#lib/domain/types.ts';
	import RawDataModal from '#lib/features/measurements/RawDataModal.svelte';

	/**
	 * Opens the raw measurements dialog with its filters set, such as a device's or a value's.
	 * The one way into raw data and its CSV export, wherever it is offered.
	 */
	let {
		query,
		label = 'Raw data',
		primary = false,
		class: className = ''
	}: {
		query: RawQuery;
		label?: string;
		/** The main action of a page header; otherwise a quiet button. */
		primary?: boolean;
		class?: string;
	} = $props();

	let open = $state(false);
</script>

<button
	type="button"
	class={['btn', primary ? 'btn-primary' : 'btn-ghost', className]}
	aria-haspopup="dialog"
	onclick={() => (open = true)}
>
	{#if primary}<Icon name="download" class="size-4" />{/if}
	{label}
</button>
{#if open}<RawDataModal bind:open {query} />{/if}
