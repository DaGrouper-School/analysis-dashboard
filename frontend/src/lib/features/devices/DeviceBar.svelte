<script lang="ts">
	import type { Snippet } from 'svelte';
	import DevicePicker, { type PickableDevice } from '#lib/features/devices/DevicePicker.svelte';

	/**
	 * The device picker, with the page's own buttons beside it on a wide screen and under it on a
	 * phone: on the dashboard, pairing a new device and the classroom's Classrooms page.
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
	<div class={['flex flex-wrap items-center gap-3', className]}>
		<DevicePicker class="max-lg:w-full" {devices} {deviceId} {onpick} />
		<div class="flex flex-wrap gap-2">{@render actions()}</div>
	</div>
{:else}
	<DevicePicker class={className} {devices} {deviceId} {onpick} />
{/if}
