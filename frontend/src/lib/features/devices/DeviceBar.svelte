<script lang="ts">
	import type { Snippet } from 'svelte';
	import DevicePicker, { type PickableDevice } from '#lib/features/devices/DevicePicker.svelte';

	/**
	 * The device picker, with the page's own buttons at the row's right edge: on the dashboard,
	 * pairing a new device (just its icon on a phone).
	 */
	let {
		devices,
		deviceId,
		onpick,
		actions,
		class: className = ''
	}: {
		devices: PickableDevice[];
		deviceId: string | undefined;
		onpick: (id: string) => void;
		actions?: Snippet;
		class?: string;
	} = $props();
</script>

{#if actions}
	<!-- On a wide screen, the same three columns as the cards below: the picker as wide as the first. -->
	<div class={['flex items-end gap-3 lg:grid lg:grid-cols-3 lg:gap-5', className]}>
		<DevicePicker class="lg:w-full" {devices} {deviceId} {onpick} />
		<!-- Its buttons as tall as the dropdown beside them, square on a phone. -->
		<div
			class="flex shrink-0 gap-2 sm:ml-auto lg:col-start-3 lg:justify-self-end [&_.btn]:h-12 max-sm:[&_.btn]:w-12"
		>
			{@render actions()}
		</div>
	</div>
{:else}
	<DevicePicker class={className} {devices} {deviceId} {onpick} />
{/if}
