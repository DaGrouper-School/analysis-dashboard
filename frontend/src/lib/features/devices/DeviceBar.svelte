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
	<div class={['flex items-end gap-3', className]}>
		<DevicePicker {devices} {deviceId} {onpick} />
		<!-- Its buttons as tall as the dropdown beside them, square on a phone. -->
		<div class="flex shrink-0 gap-2 sm:ml-auto [&_.btn]:h-12 max-sm:[&_.btn]:w-12">
			{@render actions()}
		</div>
	</div>
{:else}
	<DevicePicker class={className} {devices} {deviceId} {onpick} />
{/if}
