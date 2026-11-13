<script lang="ts" module>
	/** A device to pick, with its registry colour and icon. */
	export interface PickableDevice {
		id: string;
		name: string;
		color: string;
		icon: string;
	}
</script>

<script lang="ts">
	import Select from '#lib/components/Select.svelte';

	/**
	 * The classroom's devices as a labelled dropdown (AP-45), each with its registry icon and
	 * colour. The picked one sets what "What's happening now?" and the domains show, and the
	 * page's colour. The same on the public classroom page, the dashboard and Domains. It always
	 * shows, with no devices too, so every classroom's page has the same layout.
	 */
	let {
		devices,
		deviceId,
		onpick,
		class: className = ''
	}: {
		devices: PickableDevice[];
		deviceId: string | undefined;
		onpick: (id: string) => void;
		class?: string;
	} = $props();

	const options = $derived(
		devices.map((d) => ({ value: d.id, label: d.name, icon: d.icon, color: d.color }))
	);
</script>

<Select
	{options}
	value={deviceId}
	label="Device"
	placeholder={devices.length ? 'Choose a device' : 'No devices yet'}
	disabled={!devices.length}
	class="min-w-0 flex-1 sm:w-96 sm:flex-none {className}"
	onchange={onpick}
/>
