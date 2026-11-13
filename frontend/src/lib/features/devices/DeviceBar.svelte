<script lang="ts">
	import type { Snippet } from 'svelte';
	import DevicePicker, { type PickableDevice } from '#lib/features/devices/DevicePicker.svelte';

	/**
	 * The device picker at the top of What's happening now, with the page's own buttons at the
	 * row's right edge: on the dashboard, pairing a new device (just its icon on a phone).
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

<!-- Inside What's happening now: the picker takes the free width, its buttons at the right. -->
<div class={['flex items-end gap-3', className]}>
	<DevicePicker fill {devices} {deviceId} {onpick} />
	{#if actions}
		<!-- Its buttons as tall as the dropdown beside them, square on a phone. -->
		<div class="flex shrink-0 gap-2 [&_.btn]:h-12 max-sm:[&_.btn]:w-12">
			{@render actions()}
		</div>
	{/if}
</div>
